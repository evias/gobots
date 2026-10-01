package conn

import "github.com/evias/gobots/botfile"

// XXX
func NewTransport(
	conf botfile.ConnectionConfig,
	options ...TransportOption,
) Transport {
	switch {
	case conf.Type == "serial":
		return NewSerialTransport(conf, options...)
	case conf.Type == "ble":
	case conf.Type == "bluetooth":
		return NewBLETransport(conf, options...)
	default:
	}
	return NewTCPTransport(conf, options...)
}
