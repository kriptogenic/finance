package mcp

type config struct {
	name    string
	version string
}

type Option func(*config)

// Name sets the server name advertised to clients. Empty is ignored.
func Name(name string) Option {
	return func(c *config) {
		if name != "" {
			c.name = name
		}
	}
}

// Version sets the server version advertised to clients. Empty is ignored.
func Version(version string) Option {
	return func(c *config) {
		if version != "" {
			c.version = version
		}
	}
}
