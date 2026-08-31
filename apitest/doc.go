// Package apitest provides e2e API testing on top of `testing.T`. Fully code-configured
// - no env var or yaml reading inside the SDK. Caller loads env vars/secrets and
// passes them via options (single target).
//
// Usage:
//
//	func TestUserLifecycle(t *testing.T) {
//	    api := apitest.New(t,
//	        apitest.WithBaseURL(os.Getenv("API_BASE_URL")),
//	        apitest.WithCognito(cognito.Config{
//	            UserPoolID: os.Getenv("COGNITO_USER_POOL_ID"),
//	            ClientID: os.Getenv("COGNITO_CLIENT_ID"),
//	            Username: os.Getenv("COGNITO_USERNAME"),
//	            Password: os.Getenv("COGNITO_PASSWORD"),
//	        }),
//	        apitest.WithOpenAPISpec("openapi/openapi.yaml"), // optional, validates METHOD PATH
//	    )
//	    var id string
//	    api.POST(t, "/users", apitest.WithBody(map[string]any{"email":"a@b.com"})).ExpectStatus("201").Capture("$.id",&id)
//	    api.GET(t, "/users/{id}", apitest.WithPath("id", id)).ExpectStatusCode(200)
//	}
package apitest
