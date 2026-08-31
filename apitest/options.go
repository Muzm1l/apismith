package apitest

// CallOption configures a single request. Go's type system ensures
// body/query/path are checked at compile time.
type CallOption func(*callConfig)

type callConfig struct {
	Body     any
	BodyFile string
	Path     map[string]string
	Query    map[string]string
	Headers  map[string]string
	NoAuth   bool
}

// WithBody sets JSON body. Pass string/[]byte for raw, or any struct/map which is json.Marshal'd.
// Using typed structs (e.g. backend's CreateUserRequest) gives compile-time safety.
func WithBody(v any) CallOption {
	return func(c *callConfig) { c.Body = v }
}

// WithBodyFile reads body from file.
func WithBodyFile(p string) CallOption {
	return func(c *callConfig) { c.BodyFile = p }
}

// WithPath sets one path param (e.g. WithPath("id", "123") for /users/{id}).
func WithPath(k, v string) CallOption {
	return func(c *callConfig) {
		if c.Path == nil {
			c.Path = map[string]string{}
		}
		c.Path[k] = v
	}
}

// WithPathMap sets multiple path params.
func WithPathMap(m map[string]string) CallOption {
	return func(c *callConfig) {
		if c.Path == nil {
			c.Path = map[string]string{}
		}
		for k, v := range m {
			c.Path[k] = v
		}
	}
}

// WithQuery adds a query param.
func WithQuery(k, v string) CallOption {
	return func(c *callConfig) {
		if c.Query == nil {
			c.Query = map[string]string{}
		}
		c.Query[k] = v
	}
}

// WithQueryMap sets multiple query params.
func WithQueryMap(m map[string]string) CallOption {
	return func(c *callConfig) {
		if c.Query == nil {
			c.Query = map[string]string{}
		}
		for k, v := range m {
			c.Query[k] = v
		}
	}
}

// WithHeader adds a header.
func WithHeader(k, v string) CallOption {
	return func(c *callConfig) {
		if c.Headers == nil {
			c.Headers = map[string]string{}
		}
		c.Headers[k] = v
	}
}

// WithRequestNoAuth skips JWT injection for this request (for public endpoints like POST /auth/login).
func WithRequestNoAuth() CallOption {
	return func(c *callConfig) { c.NoAuth = true }
}
