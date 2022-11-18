package registry

import "github.com/busgo/elsa/registry/transport"

type config struct {
	transport   transport.RegistryTransport
	dialTimeout int64
	retryPeriod int64
}

type Option func(*config)

var _config = config{
	dialTimeout: 5,
	retryPeriod: 5,
}

func WithTransport(transport transport.RegistryTransport) Option {
	return func(c *config) {
		c.transport = transport
	}
}

func WithDialTimeout(dailTimeout int64) Option {
	return func(c *config) {
		c.dialTimeout = dailTimeout
	}
}

func WithretryPeriod(retryPeriod int64) Option {
	return func(c *config) {
		c.retryPeriod = retryPeriod
	}
}
