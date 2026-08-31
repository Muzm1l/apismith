package apitest

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"aegion-dynamic/apismith/openapi"
	"aegion-dynamic/apismith/request"
)

// Response wraps request.ExecuteOutput with go-test assertions.
// Every Expect* calls tb.Helper and tb.Fatalf on failure, so `go test -v`
// shows exactly which assertion broke, with status/duration/body.
type Response struct {
	tb         testing.TB
	client     *Client
	ep         *openapi.Endpoint
	in         request.ExecuteInput
	out        request.ExecuteOutput
	prettyBody string
}

// Raw returns the underlying ExecuteOutput (for custom checks).
func (r *Response) Raw() request.ExecuteOutput { return r.out }

// Status returns HTTP status code.
func (r *Response) Status() int { return r.out.Status }

// Body returns pretty-printed body (JSON indented if applicable).
func (r *Response) Body() string { return r.prettyBody }

// Endpoint returns the resolved OpenAPI endpoint.
func (r *Response) Endpoint() *openapi.Endpoint { return r.ep }

// ExpectStatus asserts status. Want examples: "200", "201", "2xx", "" (any 2xx).
// Like `apismith call --expect`. Fails test on mismatch.
func (r *Response) ExpectStatus(want string) *Response {
	r.tb.Helper()
	want = strings.TrimSpace(want)
	if want == "" {
		want = "2xx"
	}
	if r.out.Error != "" {
		r.tb.Fatalf("apitest: %s %s -> transport error: %s\nrequest: %+v", r.ep.Method, r.ep.Path, r.out.Error, r.in)
	}
	matched := false
	if strings.HasSuffix(strings.ToLower(want), "xx") && len(want) == 3 {
		class, _ := strconv.Atoi(want[:1])
		matched = r.out.Status/100 == class
	} else {
		code, err := strconv.Atoi(want)
		if err != nil {
			r.tb.Fatalf("apitest: invalid expect status %q", want)
		}
		matched = r.out.Status == code
	}
	if !matched {
		r.tb.Fatalf("apitest: %s %s expected status %s got %d %s\nbody: %s\nurl: %s %dms",
			r.ep.Method, r.ep.Path, want, r.out.Status, r.out.StatusText, r.prettyBody, r.out.URL, r.out.DurationMS)
	}
	return r
}

// ExpectStatusCode is typed shorthand for ExpectStatus("200").
func (r *Response) ExpectStatusCode(code int) *Response {
	r.tb.Helper()
	return r.ExpectStatus(strconv.Itoa(code))
}

// ExpectBodyContains fails if body does not contain substr.
func (r *Response) ExpectBodyContains(substr string) *Response {
	r.tb.Helper()
	if !strings.Contains(r.out.Body, substr) {
		r.tb.Fatalf("apitest: %s %s body does not contain %q\nbody: %s", r.ep.Method, r.ep.Path, substr, r.prettyBody)
	}
	return r
}

// ExpectJSON asserts a JSON path equals want. Path uses dot notation or JSONPath
// like "$.id" / "user.email" / "$.items[0].name". Want is compared after JSON marshal
// so typed values work: ExpectJSON("$.count", 3) or ExpectJSON("$.email", "a@b.com").
func (r *Response) ExpectJSON(path string, want any) *Response {
	r.tb.Helper()
	got := r.JSON(path)
	// normalize want to JSON string for comparison (handles numbers/bools)
	wantJSON, _ := json.Marshal(want)
	gotJSON, _ := json.Marshal(got)
	// special: if got is string, json.Marshal adds quotes; unwrap for direct string compare
	var gotVal any
	if err := json.Unmarshal(gotJSON, &gotVal); err != nil {
		gotVal = got
	}
	var wantVal any
	if err := json.Unmarshal(wantJSON, &wantVal); err != nil {
		wantVal = want
	}
	if fmt.Sprintf("%v", gotVal) != fmt.Sprintf("%v", wantVal) {
		// fallback to raw JSON compare
		if string(gotJSON) != string(wantJSON) {
			r.tb.Fatalf("apitest: %s %s JSON %q expected %s got %s\nbody: %s", r.ep.Method, r.ep.Path, path, string(wantJSON), string(gotJSON), r.prettyBody)
		}
	}
	return r
}

// JSON extracts a value at path. Supports "$.a.b", "a.b", "$.a[0].b".
// Returns nil if not found or body is not JSON.
func (r *Response) JSON(path string) any {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "$.")
	path = strings.TrimPrefix(path, "$")
	path = strings.TrimPrefix(path, ".")
	if path == "" {
		var v any
		if err := json.Unmarshal([]byte(r.out.Body), &v); err == nil {
			return v
		}
		return r.out.Body
	}
	var data any
	if err := json.Unmarshal([]byte(r.out.Body), &data); err != nil {
		return nil
	}
	return getPath(data, path)
}

// Capture extracts JSON path into dest *string. Fails test if path missing.
// Example: var id string; resp.Capture("$.id", &id)
func (r *Response) Capture(path string, dest *string) *Response {
	r.tb.Helper()
	if dest == nil {
		r.tb.Fatalf("apitest: Capture dest is nil")
	}
	v := r.JSON(path)
	if v == nil {
		r.tb.Fatalf("apitest: %s %s capture %q not found\nbody: %s", r.ep.Method, r.ep.Path, path, r.prettyBody)
	}
	switch x := v.(type) {
	case string:
		*dest = x
	default:
		// marshal non-strings
		b, _ := json.Marshal(x)
		// unwrap JSON string quotes
		if len(b) >= 2 && b[0] == '"' && b[len(b)-1] == '"' {
			var s string
			_ = json.Unmarshal(b, &s)
			*dest = s
		} else {
			*dest = string(b)
		}
	}
	return r
}

// Decode unmarshals the whole response body into v. Fails on invalid JSON.
func (r *Response) Decode(v any) *Response {
	r.tb.Helper()
	if err := json.Unmarshal([]byte(r.out.Body), v); err != nil {
		r.tb.Fatalf("apitest: %s %s decode: %v\nbody: %s", r.ep.Method, r.ep.Path, err, r.prettyBody)
	}
	return r
}

// RequireNoError fails if transport error happened.
func (r *Response) RequireNoError() *Response {
	r.tb.Helper()
	if r.out.Error != "" {
		r.tb.Fatalf("apitest: %s %s error: %s", r.ep.Method, r.ep.Path, r.out.Error)
	}
	return r
}

// Dump logs status/url/duration/body at tb.Log level (visible with -v).
func (r *Response) Dump() *Response {
	r.tb.Helper()
	r.tb.Logf("%s %s %d %s %dms\n%s", r.ep.Method, r.ep.Path, r.out.Status, r.out.URL, r.out.DurationMS, r.prettyBody)
	return r
}

// getPath traverses map/slice via "a.b[0].c" style.
func getPath(data any, path string) any {
	cur := data
	parts := strings.Split(path, ".")
	for _, p := range parts {
		if p == "" {
			continue
		}
		// handle a[0] or a[12]
		if idx := strings.Index(p, "["); idx != -1 {
			key := p[:idx]
			rest := p[idx:]
			// map lookup first
			if key != "" {
				m, ok := cur.(map[string]any)
				if !ok {
					return nil
				}
				cur = m[key]
			}
			// parse indices like [0][1] - we only need [0]
			for len(rest) > 0 && rest[0] == '[' {
				end := strings.Index(rest, "]")
				if end == -1 {
					return nil
				}
				numStr := rest[1:end]
				n, err := strconv.Atoi(numStr)
				if err != nil {
					return nil
				}
				arr, ok := cur.([]any)
				if !ok || n < 0 || n >= len(arr) {
					return nil
				}
				cur = arr[n]
				rest = rest[end+1:]
			}
		} else {
			m, ok := cur.(map[string]any)
			if !ok {
				return nil
			}
			cur = m[p]
		}
		if cur == nil {
			return nil
		}
	}
	return cur
}
