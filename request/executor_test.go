package request

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResolveBaseURL(t *testing.T) {
	got, err := ResolveBaseURL("http://localhost:8080/api/v1/", "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "http://localhost:8080/api/v1" {
		t.Fatalf("trim: %s", got)
	}
	got, err = ResolveBaseURL("http://localhost:8080/api/v1", "https://api.example.com/api/v1/")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://api.example.com/api/v1" {
		t.Fatalf("override: %s", got)
	}
	if _, err := ResolveBaseURL("", ""); err == nil {
		t.Fatal("expected error for empty url")
	}
	if _, err := ResolveBaseURL("not-a-url", ""); err == nil {
		t.Fatal("expected error for missing scheme")
	}
	if _, err := ResolveBaseURL("ftp://example.com", ""); err == nil {
		t.Fatal("expected error for non-http scheme")
	}
}

func TestBuildURL(t *testing.T) {
	u, err := BuildURL("http://localhost:8080/api/v1", "/users/{id}", map[string]string{"id": "abc"}, map[string]string{"page": "1"})
	if err != nil {
		t.Fatal(err)
	}
	if u != "http://localhost:8080/api/v1/users/abc?page=1" {
		t.Fatalf("got %s", u)
	}
}

func TestMaskHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("Authorization", "Bearer super-secret-token")
	h.Set("Content-Type", "application/json")
	masked := MaskHeaders(h)
	if masked["Authorization"] != "Bearer ********" {
		t.Fatalf("authorization not masked: %q", masked["Authorization"])
	}
	if masked["Content-Type"] != "application/json" {
		t.Fatalf("content-type changed: %q", masked["Content-Type"])
	}
}

func TestExecute(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("auth header: %q", r.Header.Get("Authorization"))
		}
		if r.URL.Path != "/api/v1/users/9" {
			t.Errorf("path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if strings.TrimSpace(string(body)) != `{"name":"Ada"}` {
			t.Errorf("body: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"9","name":"Ada"}`))
	}))
	defer srv.Close()

	exec := NewExecutor()
	out := exec.Execute(ExecuteInput{
		Method:     "POST",
		Path:       "/users/{id}",
		PathParams: map[string]string{"id": "9"},
		Body:       `{"name":"Ada"}`,
		AuthMode:   AuthJWT,
		JWT:        "tok",
	}, srv.URL+"/api/v1", false)

	if out.Status != 201 {
		t.Fatalf("status %d err=%s", out.Status, out.Error)
	}
	if !strings.Contains(out.Body, "Ada") {
		t.Fatalf("body %s", out.Body)
	}
	if out.RequestMasked["Authorization"] != "Bearer ********" {
		t.Fatalf("request auth leaked: %+v", out.RequestMasked)
	}
}

func TestProductionGuard(t *testing.T) {
	exec := NewExecutor()
	out := exec.Execute(ExecuteInput{Method: "DELETE", Path: "/users/1"}, "http://example", true)
	if out.Error == "" {
		t.Fatal("expected production confirmation error")
	}
}

func TestPrettyJSON(t *testing.T) {
	got := PrettyJSON(`{"a":1}`)
	if !strings.Contains(got, "\n") {
		t.Fatalf("expected indent, got %q", got)
	}
}
