# apitest

Go SDK for end to end API testing on top of `testing.T` and `go test`. No yaml, no env var reading inside the SDK. You load secrets in your app and pass them via code. Single base URL per client.

Built on apismith internals: `request.Executor` for HTTP, `openapi.Catalog` for optional validation, `auth/cognito` for JWT.

## Install

```sh
go get aegion-dynamic/apismith/apitest
```

## Quick start

```go
package e2e_test

import (
    "fmt"
    "os"
    "testing"
    "time"

    "aegion-dynamic/apismith/apitest"
    "aegion-dynamic/apismith/auth/cognito"
)

func newAPI(t *testing.T) *apitest.Client {
    t.Helper()
    return apitest.New(t,
        apitest.WithBaseURL(os.Getenv("API_BASE_URL")),
        apitest.WithCognito(cognito.Config{
            UserPoolID: os.Getenv("COGNITO_USER_POOL_ID"),
            ClientID:   os.Getenv("COGNITO_CLIENT_ID"),
            Username:   os.Getenv("COGNITO_USERNAME"),
            Password:   os.Getenv("COGNITO_PASSWORD"),
        }),
        apitest.WithOpenAPISpec("openapi/openapi.yaml"), // optional, validates METHOD PATH
    )
}

func TestUserLifecycle(t *testing.T) {
    api := newAPI(t)

    email := fmt.Sprintf("e2e+%d@example.com", time.Now().UnixNano())
    var id string

    t.Run("create", func(t *testing.T) {
        api.POST(t, "/users", apitest.WithBody(map[string]any{"email": email, "name": "e2e"})).
            ExpectStatus("201").
            Capture("$.id", &id)
    })

    t.Run("get", func(t *testing.T) {
        api.GET(t, "/users/{id}", apitest.WithPath("id", id)).
            ExpectStatusCode(200).
            ExpectJSON("$.email", email)
    })

    t.Run("concrete", func(t *testing.T) {
        api.GET(t, "/users/"+id).ExpectStatus("2xx")
    })
}
```

Run:

```sh
API_BASE_URL=http://localhost:8080/api/v1 \
COGNITO_USER_POOL_ID=ap-south-1_xxx \
COGNITO_CLIENT_ID=xxx \
COGNITO_USERNAME=user@example.com \
COGNITO_PASSWORD=secret \
go test ./e2e -v
```

Or use any env loading you prefer in your app (godotenv, etc.) and pass to `WithBaseURL`/`WithCognito`.

## Configuration

All config is via code options. No env vars are read inside the SDK.

* `WithBaseURL(url)` required. Base URL for all requests, e.g. `http://localhost:8080/api/v1`.
* `WithCognito(cognito.Config{...})` optional. If set, JWT is minted lazily on first authenticated request via Cognito SRP and cached. Requires `UserPoolID`, `ClientID`, `Username`, `Password`. Optional fields: `ClientSecret`, `Region` (default `ap-south-1`), `Endpoint` (for emulator), `AuthFlow`.
* `WithOpenAPISpec(path)` optional. If set, validates `METHOD PATH` against the spec and enables `CallByID`. If unset, requests pass through without validation.
* `WithJWT(token)` optional. Bypass Cognito with a fixed token (useful for tests with `httptest`).
* `WithClientNoAuth()` optional. Disable JWT for all requests.
* `WithExecutor(*request.Executor)` optional. Custom HTTP client.

## Requests

```go
api.GET(t, "/users", apitest.WithQuery("page", "1"))
api.POST(t, "/users", apitest.WithBody(map[string]any{"email": "a@b.com"}))
api.POST(t, "/users", apitest.WithBody(MyStruct{Email: "a@b.com"})) // typesafe via json.Marshal
api.POST(t, "/users", apitest.WithBodyFile("testdata/user.json"))
api.GET(t, "/users/{id}", apitest.WithPath("id", id))
api.GET(t, "/users/"+id) // concrete path also works
api.POST(t, "/auth/login", apitest.WithRequestNoAuth(), apitest.WithBody(...))
api.CallByID(t, "createUser", apitest.WithBody(...)) // requires WithOpenAPISpec
```

Per request options: `WithBody(any)`, `WithBodyFile`, `WithPath`, `WithPathMap`, `WithQuery`, `WithQueryMap`, `WithHeader`, `WithRequestNoAuth`.

## Assertions

All helpers use `t.Helper` and fail with `t.Fatalf` showing status, URL, duration and body.

```go
resp := api.POST(t, "/users", apitest.WithBody(...))
resp.ExpectStatus("201")        // exact: "201", "404", or class: "2xx"
resp.ExpectStatusCode(200)      // shorthand
resp.ExpectBodyContains("token")
resp.ExpectJSON("$.email", "a@b.com") // dot path, supports $.a.b, $.items[0].name
resp.JSON("$.id")                // extract any value
resp.Capture("$.id", &id)       // extract into Go var
resp.Decode(&myStruct)           // unmarshal full body
resp.Dump()                      // log status/URL/body with -v
```

## Table driven and parallel

Standard `go test` features work. JWT is cached and thread safe.

```go
func TestList_Table(t *testing.T) {
    api := newAPI(t)
    for _, page := range []string{"1", "2"} {
        t.Run(page, func(t *testing.T) {
            t.Parallel()
            api.GET(t, "/users", apitest.WithQuery("page", page)).ExpectStatus("2xx")
        })
    }
}
```

Filter: `go test -run TestUserLifecycle/create -v`
JSON output for CI: `go test ./e2e -json`

## Mock with httptest

```go
srv := httptest.NewServer(http.HandlerFunc(...))
api := apitest.New(t,
    apitest.WithBaseURL(srv.URL),
    apitest.WithJWT("fake"),
    apitest.WithOpenAPISpec("openapi/openapi.yaml"),
)
```

No Cognito call is made when `WithJWT` is set.

## Layout

```
apitest/client.go   New, WithBaseURL, WithCognito, WithOpenAPISpec, Do, GET/POST/PUT/PATCH/DELETE
apitest/options.go  WithBody, WithPath, WithQuery, WithHeader, WithRequestNoAuth
apitest/response.go ExpectStatus, ExpectJSON, Capture, Decode, JSON
```
