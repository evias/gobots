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
	default:
	}
	return NewTCPTransport(conf, options...)
}
