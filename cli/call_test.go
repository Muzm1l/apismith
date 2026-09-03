package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aegion-dynamic/apismith/auth/cognito"
	"aegion-dynamic/apismith/openapi"
	"aegion-dynamic/apismith/request"
)

func TestParseKV(t *testing.T) {
	got, err := parseKV([]string{"id=123", " name =x"})
	if err != nil {
		t.Fatal(err)
	}
	if got["id"] != "123" || got["name"] != "x" {
		t.Fatalf("%v", got)
	}
	if _, err := parseKV([]string{"nocolon"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestStatusMatches(t *testing.T) {
	ok200 := request.ExecuteOutput{Status: 200, OK: true}
	if !statusMatches(ok200, "") || !statusMatches(ok200, "200") || !statusMatches(ok200, "2xx") {
		t.Fatal("200 should match default, 200, and 2xx")
	}
	if statusMatches(ok200, "404") {
		t.Fatal("200 should not match 404")
	}
	notFound := request.ExecuteOutput{Status: 404, OK: false}
	if !statusMatches(notFound, "404") || statusMatches(notFound, "") {
		t.Fatal("404 matching")
	}
	failed := request.ExecuteOutput{Error: "dial error"}
	if statusMatches(failed, "") || statusMatches(failed, "200") {
		t.Fatal("transport errors must fail")
	}
}

func TestResolveCall(t *testing.T) {
	cat, err := openapi.Load(filepath.Join("..", "openapi", "testdata", "fixture.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	ep, params, err := resolveCall(cat, []string{"GET", "/users/abc"})
	if err != nil {
		t.Fatal(err)
	}
	if ep.Path != "/users/{id}" || params["id"] != "abc" {
		t.Fatalf("ep=%+v params=%v", ep, params)
	}
	ep, _, err = resolveCall(cat, []string{"createUser"})
	if err != nil || ep.Method != "POST" {
		t.Fatalf("operationId: ep=%+v err=%v", ep, err)
	}
}

func TestSelectAuthToken(t *testing.T) {
	tokens := &cognito.Tokens{AccessToken: "access", IDToken: "id"}
	got, err := selectAuthToken(tokens, false)
	if err != nil || got != "access" {
		t.Fatalf("access: got=%q err=%v", got, err)
	}
	got, err = selectAuthToken(tokens, true)
	if err != nil || got != "id" {
		t.Fatalf("id: got=%q err=%v", got, err)
	}

	_, err = selectAuthToken(&cognito.Tokens{AccessToken: "access", IDToken: ""}, true)
	if err == nil {
		t.Fatal("expected error for empty id_token")
	}
	_, err = selectAuthToken(&cognito.Tokens{AccessToken: "", IDToken: "id"}, false)
	if err == nil {
		t.Fatal("expected error for empty access_token")
	}
	_, err = selectAuthToken(nil, false)
	if err == nil {
		t.Fatal("expected error for nil tokens")
	}
}

func TestCallBaseURLOverride(t *testing.T) {
	var gotHost, gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Host
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"source":"hosted-mock"}`))
	}))
	t.Cleanup(srv.Close)

	cfgPath := writeCallTestConfig(t, "http://127.0.0.1:1/must-not-hit")
	t.Cleanup(func() {
		jsonOut = false
		configPath = ""
	})

	cmd := newRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--config", cfgPath,
		"call", "GET", "/users",
		"--no-auth",
		"--quiet",
		"--base-url", srv.URL + "/api/v1",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("call: %v", err)
	}
	if gotPath != "/api/v1/users" {
		t.Fatalf("hosted path: %q host=%q", gotPath, gotHost)
	}
	if gotAuth != "" {
		t.Fatalf("unexpected auth header %q", gotAuth)
	}
	if gotHost == "" || gotHost == "127.0.0.1:1" {
		t.Fatalf("did not reach hosted mock, host=%q", gotHost)
	}
}

func TestCallBaseURLInvalid(t *testing.T) {
	cfgPath := writeCallTestConfig(t, "http://127.0.0.1:1/unused")
	t.Cleanup(func() {
		jsonOut = false
		configPath = ""
	})

	cmd := newRootCmd()
	errBuf := &bytes.Buffer{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{
		"--config", cfgPath,
		"call", "GET", "/users",
		"--no-auth",
		"--base-url", "ftp://example.com",
	})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected invalid base url error")
	} else if !strings.Contains(err.Error(), "http or https") {
		t.Fatalf("error: %v", err)
	}
}

func writeCallTestConfig(t *testing.T, baseURL string) string {
	t.Helper()
	spec, err := filepath.Abs(filepath.Join("..", "openapi", "testdata", "fixture.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONSOLE_OPENAPI_SPEC", spec)
	t.Setenv("CONSOLE_BASE_URL", "")
	t.Setenv("CONSOLE_DEFAULT_ENV", "dev")
	t.Setenv("CONSOLE_CONFIG", "")

	dir := t.TempDir()
	path := filepath.Join(dir, "environments.yaml")
	body := "listen: \"127.0.0.1:0\"\nopenapi_spec: \"" + spec + "\"\ndefault_environment: dev\nenvironments:\n  - id: dev\n    name: DEV\n    base_url: \"" + baseURL + "\"\n    production: false\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
