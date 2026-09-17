package conn

import "github.com/evias/gobots/botfile"

// XXX
func NewTransport(
	conf botfile.ConnectionConfig,
	options ...TransportOption,
) Transport {
	return NewTCPTransport(conf, options...)
}
