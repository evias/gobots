package conn

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"tinygo.org/x/bluetooth"

	"github.com/evias/gobots/botfile"
	apierr "github.com/evias/gobots/errors"
)

const (
	DefaultBLETimeoutMs = 3000
	DefaultBLEAttempts  = 3
)

// BLEChannel defines the contract for read/write bluetooth channels, notably
// this interface is already implemented by [bluetooth.DeviceCharacteristic]
// and is defined to avoid a direct dependency that would affect read/write.
type BLEChannel interface {
	Write(p []byte) (int, error)
	Read(bytes []byte) (int, error)
}

// ----------------------------------------------------------------------------
// BLEConn

// BLEConn defines a proxy instance for bluetooth connections.
// This structure implements [Socket] by proxying calls to the
// underlying [bluetooth.Device] and [bluetooth.DeviceCharacteristic].
//
// Example to update the active communication channel:
// ```
// conn := transport.Socket().(*BLEConn)
// conn.SetCurrentChannel(uuid)
// ```
type BLEConn struct {
	EnableDiscovery bool

	// device contains the connected [bluetooth.Device] instance.
	device *bluetooth.Device

	// servicesByUUID serves as a read-only registry filled in [BLETransport#Open].
	servicesByUUID map[bluetooth.UUID]bluetooth.DeviceService

	// channelsByUUID serves as a read-only registry filled in [BLETransport#Open].
	channelsByUUID map[bluetooth.UUID]BLEChannel

	// currentChannel contains the currently active communication channel,
	// See more: [bluetooth.DeviceCharacteristic].
	currentChannel BLEChannel
}

// Ensure that our implementation satisfies [Socket] interface.
var _ Socket = (*BLEConn)(nil)

// Write writes p to the underlying [bluetooth.DeviceCharacteristic].
// Write implements [Socket].
func (c BLEConn) Write(p []byte) (int, error) {
	return c.currentChannel.Write(p)
}

// Read reads bytes from the underlying [bluetooth.DeviceCharacteristic].
// Read implements [Socket].
func (c BLEConn) Read(bytes []byte) (int, error) {
	return c.currentChannel.Read(bytes)
}

// Close disconnects from the underlying [bluetooth.Device].
// Close implements [Socket].
func (c BLEConn) Close() error {
	if c.device == nil {
		return nil
	}

	return c.device.Disconnect()
}

// SetCurrentChannel permits to switch the active communication channel to a
// specific uuid of a [bluetooth.DeviceCharacteristic].
//
// This method requires the characteristic to have been discovered before.
func (c *BLEConn) SetCurrentChannel(uuid bluetooth.UUID) error {
	var (
		channel BLEChannel
		ok      bool
	)
	if channel, ok = c.channelsByUUID[uuid]; !ok {
		return &apierr.AppError{
			Code:    apierr.ErrCharNotFound,
			Message: fmt.Sprintf("Bluetooth characteristic not found for UUID '%s'", uuid.String()),
			Cause:   nil,
		}
	}

	c.currentChannel = channel
	return nil
}

// Discover executes a Service and Characteristics discovery using a bluetooth
// connected [bluetooth.Device].
//
// It filters UUIDs using chanConfs, or defaults to returning all UUIDs
// it discovered. This method stores the discovered services and characteristics
// in the servicesByUUID and channelsByUUID properties, and it sets the
// currentChannel to the *first* discovered characteristic.
//
// Importantly, you may need to call [BLEConn#SetCurrentChannel] if you want
// to communicate using a different bluetooth characteristic than the first.
func (c *BLEConn) Discover(chanConfs []botfile.BluetoothChannelConfig) error {
	var (
		services []bluetooth.DeviceService
		allChars []bluetooth.DeviceCharacteristic // implements BLEChannel
		chanErr  error
	)
	filterServicesUUID,
		filterChannelsUUID := FilterUUIDs(chanConfs)

	// Filter advertised bluetooth services by UUID.
	if services, chanErr = c.device.DiscoverServices(filterServicesUUID); chanErr != nil {
		return chanErr
	}
	// Cannot continue without any services.
	if len(services) == 0 {
		return &apierr.AppError{
			Code:    apierr.ErrServiceNotFound,
			Message: "Bluetooth services not found",
			Cause:   nil,
		}
	}

	// Per-each discovered service, we request the advertised characteristics
	// and keep the ones we are interested in. By default, keep all.
	for s := 0; s < len(services); s++ {
		service := services[s]

		// BLEConn state transition protected by mtx.
		c.servicesByUUID[service.UUID()] = service

		// Filter advertised bluetooth characteristics by UUID.
		var chars []bluetooth.DeviceCharacteristic
		if chars, chanErr = service.DiscoverCharacteristics(filterChannelsUUID); chanErr != nil {
			return chanErr
		}
		if len(chars) == 0 {
			continue // to next service
		}
		allChars = append(allChars, chars...)
	}

	// Cannot continue without any characteristics.
	if len(allChars) == 0 {
		return &apierr.AppError{
			Code:    apierr.ErrCharNotFound,
			Message: "Bluetooth characteristics not found",
			Cause:   nil,
		}
	}

	// Note that at time of Open(), we set the *first* channel to be active.
	// For changing the active communication channel, see [BLEConn#SetCurrentChannel].
	for i := 0; i < len(allChars); i++ {
		channel := allChars[i]

		if i == 0 {
			c.currentChannel = channel
		}
		c.channelsByUUID[channel.UUID()] = channel
	}

	return nil
}

// ----------------------------------------------------------------------------
// BLETransport

// BLETransport implements the [Transport] interface to provide a Bluetooth
// Low Energy (BLE) communication layer.
//
// Notable method implementations include:
// - [BLETransport#Open]: Connect to a device over Bluetooth.
// - [BLETransport#Close]: Disconnect from a connected device.
// - [BLETransport#Read]: Read raw bytes from a connected device.
// - [BLETransport#Write]: Send raw bytes to a connection device.
//
// This implementation is thread-safe. An internal mutex lock is taken
// for all state transitions, thereby protecting the internal conn instance.
type BLETransport struct {
	connConf  botfile.ConnectionConfig
	chanConfs []botfile.BluetoothChannelConfig
	logger    *slog.Logger

	mtx    *sync.Mutex
	dev    *bluetooth.Adapter
	conn   *BLEConn      // under mtx
	reader *bufio.Reader // under mtx
	writer *bufio.Writer // under mtx
	dialFn DialFunc
}

// Ensure that our implementation satisfies interface.
var _ Transport = (*BLETransport)(nil)

// NewBLETransport creates a new Transport instance around a [botfile.ConnectionConfig].
func NewBLETransport(
	cfg botfile.ConnectionConfig,
	options ...TransportOption,
) Transport {
	blet := &BLETransport{
		mtx: new(sync.Mutex),

		dev:       bluetooth.DefaultAdapter,
		connConf:  cfg,
		chanConfs: []botfile.BluetoothChannelConfig{},
		logger:    slog.Default(),
	}

	for _, option := range options {
		option(blet)
	}

	blet.mtx.Lock()
	defer blet.mtx.Unlock()

	return blet
}

// WithBLELogger implements an option helper to inject a custom [slog.Logger].
func WithBLELogger(logger *slog.Logger, lvl slog.Level) TransportOption {
	return func(t Transport) {
		blet := t.(*BLETransport)
		blet.logger = logger
	}
}

// WithBLEDialer implements an option helper to inject a custom [DialFunc].
func WithBLEDialer(fn DialFunc) TransportOption {
	return func(t Transport) {
		blet := t.(*BLETransport)
		blet.dialFn = fn
	}
}

// WithBluetoothChannelConfig implements an option helper to inject a custom
// bluetooth channel configuration.
//
// Use this method if you know specific services and characteristics UUIDs for
// the device you are connecting to, as pre-defined UUIDs improve connection speed.
func WithBluetoothChannelConfig(cfg ...botfile.BluetoothChannelConfig) TransportOption {
	return func(t Transport) {
		blet := t.(*BLETransport)
		blet.chanConfs = append(blet.chanConfs, cfg...)
	}
}

// Type returns the transport type, e.g. "tcp", "serial", "bluetooth".
func (*BLETransport) Type() string {
	return "bluetooth"
}

// Socket returns the opened socket instance, see io.ReadWriteCloser.
func (blet *BLETransport) Socket() Socket {
	blet.mtx.Lock()
	defer blet.mtx.Unlock()

	return blet.conn
}

// Dialer returns a dial function, or a custom dialer that uses the
// underlying Bluetooth implementation from [bluetooth.Device].
//
// Note that this dialer *scans* the bluetooth adapter and matches on
// the exact bluetooth MAC address, then connects to the device and
// uses advertised service UUID and characteristics UUID as defined.
func (blet *BLETransport) Dialer() DialFunc {
	// use custom dialer process as injected
	if blet.dialFn != nil {
		return blet.dialFn
	}

	// fallback to [bluetooth.Device#Connect] implementation
	return func(ctx context.Context, network, addr string) (Socket, error) {
		// Make sure we enable the bluetooth adapter pre-scan.
		blet.dev.Enable()

		// Runs a bluetooth scan to find address.
		scanCh := make(chan bluetooth.ScanResult, 1)
		scanErr := blet.dev.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
			blet.logger.Debug(fmt.Sprintf("Found bluetooth device '%s'", result.Address.String()),
				"rssi", result.RSSI,
				"name", result.LocalName(),
				"srch", blet.String(),
			)

			// TODO(evias): Random addresses, lowercase/uppercase, non-exact matches.
			if result.Address.String() == blet.String() {
				adapter.StopScan()
				scanCh <- result
			}
		})
		if scanErr != nil {
			return nil, scanErr
		}

		// Connect to bluetooth device, matched by exact address.
		var (
			device  bluetooth.Device
			connErr error
		)
		select {
		case <-ctx.Done(): // Dialer may timeout given it exceeds allowed duration.
			return nil, &apierr.AppError{
				Code:    apierr.ErrContextTimeout,
				Message: "Connection context timed out",
				Cause:   nil,
			}

		case result := <-scanCh: // Scan result found by exact address.
			device, connErr = blet.dev.Connect(
				result.Address,
				bluetooth.ConnectionParams{},
			)
			if connErr != nil {
				return nil, connErr
			}
		}

		// Connection established.
		conn := &BLEConn{
			EnableDiscovery: true, // always enabled, except in tests.
			device:          &device,
		}
		return conn, nil
	}
}

// Addr returns the remote address or nil, i.e. [net.Conn#RemoteAddr].
func (blet *BLETransport) Addr() string {
	return blet.connConf.Host
}

// String returns a string representation of the instance.
func (blet *BLETransport) String() string {
	return blet.Addr()
}

// Open dials the remote address, i.e. connect to the remote.
func (blet *BLETransport) Open() error {
	if blet.connConf.TimeoutMs == 0 {
		blet.connConf.TimeoutMs = DefaultBLETimeoutMs
	}

	if blet.connConf.MaxAttempts == 0 {
		blet.connConf.MaxAttempts = DefaultBLEAttempts
	}

	if len(blet.connConf.Host) == 0 {
		return &apierr.AppError{
			Code:    apierr.ErrInvalidConnection,
			Message: "Missing bluetooth MAC address",
			Cause:   nil,
		}
	}

	var (
		hostWithPort = blet.String()
		dialerFn     = blet.Dialer()
		socket       Socket
		conn         *BLEConn
		err          error
	)

	for i := 0; i < int(blet.connConf.MaxAttempts); i++ {
		at := i + 1
		blet.logger.Debug(fmt.Sprintf("Connecting to serial port %s", hostWithPort),
			"attempts", at,
		)

		timeoutDuration := time.Duration(blet.connConf.TimeoutMs) * time.Millisecond
		dialCtx, cancelFn := context.WithTimeout(context.Background(), timeoutDuration)

		// XXX dialerFn should be called in a goroutine to avoid blocking main thread.
		socket, err = dialerFn(dialCtx, "ble", hostWithPort)
		cancelFn() // cancel context directly after sync-call of dialer func

		if err == nil {
			conn = socket.(*BLEConn)
			break
		}

		blet.logger.Error("Failed connection attempt",
			"host", hostWithPort, "attempts", at,
			"err", err.Error(),
		)
		// continue
	}

	if err != nil {
		return &apierr.AppError{
			Code:    apierr.ErrInvalidConnection,
			Message: fmt.Sprintf("Connection failed after %d attempts", blet.connConf.MaxAttempts),
			Cause:   err,
		}
	}

	// Bluetooth communication channels may be many. So we must request what
	// is being advertised by the [bluetooth.Device], i.e. requesting services
	// and characteristics UUIDs. Each characteristic is a potential stream.
	//
	// Given a limited list of [botfile.BluetoothChannelConfig], we shall only
	// discover services and characteristics we are interested in. The default,
	// when chanConfs is empty, is to query all services and characteristics.

	// TODO(evias): Reduce tests cross pollination/pollution. Mock discovery.
	if conn.EnableDiscovery {
		if err := conn.Discover(blet.chanConfs); err != nil {
			return &apierr.AppError{
				Code:    apierr.ErrInvalidConnection,
				Message: fmt.Sprintf("Connection failed after %d attempts", blet.connConf.MaxAttempts),
				Cause:   err,
			}
		}
	}

	blet.mtx.Lock()
	defer blet.mtx.Unlock()

	blet.conn = conn
	blet.reader = bufio.NewReader(conn)
	blet.writer = bufio.NewWriter(conn)
	return nil
}

// Close closes the connection to the remote.
func (blet *BLETransport) Close() error {
	blet.mtx.Lock()
	defer blet.mtx.Unlock()

	if blet.conn == nil {
		return nil // conn already closed
	}

	blet.reader = nil
	blet.writer = nil
	return blet.conn.Close()
}

// Read reads bytes from the stream, p must be pre-allocated.
func (blet *BLETransport) Read(
	bytes []byte,
) (int, error) {
	blet.mtx.Lock()
	defer blet.mtx.Unlock()

	if blet.reader == nil {
		return 0, net.ErrClosed
	}
	return blet.reader.Read(bytes)
}

// Write sends bytes to the stream, p must be pre-allocated.
func (blet *BLETransport) Write(p []byte) (int, error) {
	blet.mtx.Lock()
	defer blet.mtx.Unlock()

	if blet.writer == nil {
		return 0, net.ErrClosed
	}
	if _, err := blet.writer.Write(p); err != nil {
		return 0, err
	}
	if err := blet.writer.Flush(); err != nil {
		return 0, err
	}
	return len(p), nil
}

// ----------------------------------------------------------------------------
// Private implementation

// FilterUUIDs returns two slices of [bluetooth.UUID] values, that respectively
// contain the list of services UUID and the list of characteristics UUID as
// configured in chanConfs, i.e. through calling [WithBluetoothChannelConfig].
func FilterUUIDs(chanConfs []botfile.BluetoothChannelConfig) (
	filterServicesUUID []bluetooth.UUID,
	filterChannelsUUID []bluetooth.UUID,
) {
	filterServicesUUID = make([]bluetooth.UUID, len(chanConfs))
	filterChannelsUUID = make([]bluetooth.UUID, len(chanConfs))
	for c := 0; c < len(chanConfs); c++ {
		var (
			svcUUID bluetooth.UUID
			chrUUID bluetooth.UUID
			uuidErr error
		)
		if svcUUID, uuidErr = bluetooth.ParseUUID(chanConfs[c].ServiceUUID); uuidErr != nil {
			continue // CAUTION: ignores malformed UUIDs
		}
		if chrUUID, uuidErr = bluetooth.ParseUUID(chanConfs[c].CharacteristicUUID); uuidErr != nil {
			continue // CAUTION: ignores malformed UUIDs
		}

		filterServicesUUID = append(filterServicesUUID, svcUUID)
		filterChannelsUUID = append(filterChannelsUUID, chrUUID)
	}
	return // filterServicesUUID, filterChannelsUUID
}
