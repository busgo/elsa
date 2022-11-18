package transport

type config struct {
	endpoints   []string
	username    string
	password    string
	ttl         int64
	dialTimeout int64
}

var _config = config{
	endpoints:   []string{"127.0.0.1:2379"},
	username:    "",
	password:    "",
	ttl:         10,
	dialTimeout: 5,
}

type Option func(*config)

func WithEndpoints(endpoints []string) Option {

	return func(c *config) {
		c.endpoints = endpoints
	}
}

func WithUsername(username string) Option {
	return func(c *config) {
		c.username = username
	}
}

func WithPassword(password string) Option {
	return func(c *config) {
		c.password = password
	}
}

func WithTTL(ttl int64) Option {
	return func(c *config) {
		c.ttl = ttl
	}
}

func WithDialTimeout(dialTimeout int64) Option {
	return func(c *config) {
		c.dialTimeout = dialTimeout
	}
}
