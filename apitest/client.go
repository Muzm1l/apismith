package apitest

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"aegion-dynamic/apismith/auth/cognito"
	"aegion-dynamic/apismith/openapi"
	"aegion-dynamic/apismith/request"
)

// Client is the e2e test client. Fully code-configured - no env var or yaml
// reading inside the SDK. Caller loads env vars/secrets in their app and passes
// via options. Single target, single base URL.
//
// Example:
//
//	api := apitest.New(t,
//	    apitest.WithBaseURL(os.Getenv("API_BASE_URL")),
//	    apitest.WithCognito(cognito.Config{UserPoolID: ..., ClientID: ..., Username: ..., Password: ...}),
//	    apitest.WithOpenAPISpec("../openapi/openapi.yaml"), // optional, validates METHOD PATH
//	)
//	api.POST(t, "/users", apitest.WithBody(...)).ExpectStatus("201")
type Client struct {
	tb      testing.TB
	catalog *openapi.Catalog // nil when no spec
	exec    *request.Executor
	baseURL string
	cogCfg  *cognito.Config
	jwt     string
	jwtErr  error
	didAuth bool
}

// Option configures Client creation.
type Option func(*clientConfig)

type clientConfig struct {
	baseURL       string
	openAPISpec   string
	cognitoConfig *cognito.Config
	jwt           string
	executor      *request.Executor
	noAuth        bool
}

// WithBaseURL sets the API base URL (required).
func WithBaseURL(url string) Option { return func(c *clientConfig) { c.baseURL = url } }

// WithOpenAPISpec sets the OpenAPI spec path for METHOD PATH validation.
// If unset, validation is skipped (passthrough).
func WithOpenAPISpec(path string) Option { return func(c *clientConfig) { c.openAPISpec = path } }

// WithCognito sets the Cognito config used to mint JWTs lazily.
// Pass the same struct you use for auth (UserPoolID, ClientID, Username, Password, etc.).
func WithCognito(cfg cognito.Config) Option {
	return func(c *clientConfig) {
		cp := cfg
		cp.Normalize()
		c.cognitoConfig = &cp
	}
}

// WithCognitoConfig is an alias for WithCognito.
func WithCognitoConfig(cfg cognito.Config) Option { return WithCognito(cfg) }

// WithJWT bypasses Cognito and uses this token (useful for tests/fake JWT).
func WithJWT(tok string) Option {
	return func(c *clientConfig) {
		c.jwt = tok
		c.noAuth = false
	}
}

// WithClientNoAuth disables JWT injection client-wide.
func WithClientNoAuth() Option { return func(c *clientConfig) { c.noAuth = true } }

// WithExecutor injects a custom http client (e.g. for mocking).
func WithExecutor(e *request.Executor) Option { return func(c *clientConfig) { c.executor = e } }

// New creates a Client. All configuration is via code - no env vars read here.
// Caller is responsible for loading any env vars and passing them in.
func New(tb testing.TB, opts ...Option) *Client {
	tb.Helper()
	cc := &clientConfig{}
	for _, o := range opts {
		o(cc)
	}
	baseURL := strings.TrimSpace(cc.baseURL)
	if baseURL == "" {
		tb.Fatalf("apitest: WithBaseURL is required (e.g. apitest.WithBaseURL(os.Getenv(\"API_BASE_URL\")))")
	}
	var catalog *openapi.Catalog
	if strings.TrimSpace(cc.openAPISpec) != "" {
		cat, err := openapi.Load(cc.openAPISpec)
		if err != nil {
			tb.Fatalf("apitest: load OpenAPI %s: %v", cc.openAPISpec, err)
		}
		catalog = cat
	}
	exec := cc.executor
	if exec == nil {
		exec = request.NewExecutor()
	}
	c := &Client{
		tb:      tb,
		catalog: catalog,
		exec:    exec,
		baseURL: strings.TrimRight(baseURL, "/"),
		cogCfg:  cc.cognitoConfig,
	}
	if cc.noAuth {
		c.didAuth = true
		c.jwt = ""
	} else if cc.jwt != "" {
		c.jwt = cc.jwt
		c.didAuth = true
	}
	return c
}

// BaseURL returns the target base URL.
func (c *Client) BaseURL() string { return c.baseURL }

// ensureJWT mints a JWT lazily from WithCognito config, or returns the WithJWT token.
func (c *Client) ensureJWT() string {
	if c.didAuth {
		return c.jwt
	}
	c.didAuth = true
	if c.cogCfg == nil {
		return ""
	}
	if err := c.cogCfg.Validate(); err != nil {
		// no creds is okay for public endpoints - just leave jwt empty
		return ""
	}
	tokens, err := cognito.Generate(context.Background(), *c.cogCfg)
	if err != nil {
		c.jwtErr = err
		return ""
	}
	c.jwt = tokens.AccessToken
	return c.jwt
}

// Do executes a request. If OpenAPI spec was provided via WithOpenAPISpec, validates METHOD PATH.
func (c *Client) Do(tb testing.TB, method, path string, opts ...CallOption) *Response {
	tb.Helper()
	co := &callConfig{
		Headers: map[string]string{},
		Path:    map[string]string{},
		Query:   map[string]string{},
	}
	for _, o := range opts {
		o(co)
	}
	var ep *openapi.Endpoint
	var extracted map[string]string
	if c.catalog != nil {
		var err error
		ep, extracted, err = c.catalog.LookupMethodPath(method, path)
		if err != nil {
			if !strings.Contains(path, "/") && strings.TrimSpace(method) != "" && len(opts) == 0 {
				ep2, _, err2 := c.catalog.Lookup(method)
				if err2 == nil {
					ep = ep2
					extracted = map[string]string{}
					err = nil
				}
			}
			if err != nil {
				tb.Fatalf("apitest: %v", err)
			}
		}
		for k, v := range extracted {
			if _, ok := co.Path[k]; !ok {
				co.Path[k] = v
			}
		}
	} else {
		ep = &openapi.Endpoint{Method: strings.ToUpper(method), Path: path, AuthRequired: true}
		extracted = map[string]string{}
	}
	bodyStr := ""
	if co.Body != nil {
		switch v := co.Body.(type) {
		case string:
			bodyStr = v
		case []byte:
			bodyStr = string(v)
		default:
			b, err := json.Marshal(v)
			if err != nil {
				tb.Fatalf("apitest: marshal body: %v", err)
			}
			bodyStr = string(b)
		}
	} else if co.BodyFile != "" {
		b, err := os.ReadFile(co.BodyFile)
		if err != nil {
			tb.Fatalf("apitest: read bodyFile %s: %v", co.BodyFile, err)
		}
		bodyStr = string(b)
	}
	if co.Path == nil {
		co.Path = map[string]string{}
	}
	if co.Query == nil {
		co.Query = map[string]string{}
	}
	if co.Headers == nil {
		co.Headers = map[string]string{}
	}
	authMode := request.AuthJWT
	jwt := ""
	if co.NoAuth || (ep != nil && !ep.AuthRequired) {
		authMode = request.AuthNone
	} else {
		jwt = c.ensureJWT()
		if c.jwtErr != nil {
			tb.Fatalf("apitest: jwt mint failed: %v", c.jwtErr)
		}
	}
	in := request.ExecuteInput{
		Environment: "apitest",
		Method:      ep.Method,
		Path:        ep.Path,
		PathParams:  co.Path,
		Query:       co.Query,
		Headers:     co.Headers,
		Body:        bodyStr,
		AuthMode:    authMode,
		JWT:         jwt,
	}
	out := c.exec.Execute(in, c.baseURL, false)
	resp := &Response{tb: tb, in: in, ep: ep, out: out, client: c}
	if strings.Contains(strings.ToLower(out.ContentType), "json") {
		resp.prettyBody = request.PrettyJSON(out.Body)
	} else {
		resp.prettyBody = out.Body
	}
	return resp
}

// Convenience verbs
func (c *Client) GET(tb testing.TB, path string, opts ...CallOption) *Response {
	tb.Helper()
	return c.Do(tb, "GET", path, opts...)
}
func (c *Client) POST(tb testing.TB, path string, opts ...CallOption) *Response {
	tb.Helper()
	return c.Do(tb, "POST", path, opts...)
}
func (c *Client) PUT(tb testing.TB, path string, opts ...CallOption) *Response {
	tb.Helper()
	return c.Do(tb, "PUT", path, opts...)
}
func (c *Client) PATCH(tb testing.TB, path string, opts ...CallOption) *Response {
	tb.Helper()
	return c.Do(tb, "PATCH", path, opts...)
}
func (c *Client) DELETE(tb testing.TB, path string, opts ...CallOption) *Response {
	tb.Helper()
	return c.Do(tb, "DELETE", path, opts...)
}

// CallByID uses operationId (requires WithOpenAPISpec).
func (c *Client) CallByID(tb testing.TB, operationID string, opts ...CallOption) *Response {
	tb.Helper()
	if c.catalog == nil {
		tb.Fatalf("apitest: CallByID requires WithOpenAPISpec")
	}
	ep, _, err := c.catalog.Lookup(operationID)
	if err != nil {
		tb.Fatalf("apitest: %v", err)
	}
	return c.Do(tb, ep.Method, ep.Path, opts...)
}
