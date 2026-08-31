package cli

import (
	"path/filepath"
	"testing"

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
