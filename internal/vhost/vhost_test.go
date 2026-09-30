package vhost

import "testing"

const (
	apiID        = "api123"
	urlID        = "abc123"
	ipv6Loopback = "[::1]"
)

func TestFunctionURLID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		host string
		id   string
		ok   bool
	}{
		{"abc123.lambda-url.localhost", urlID, true},
		{"abc123.lambda-url.localhost:4566", urlID, true},
		{"ABC123.lambda-url.us-east-1.on.aws", urlID, true},
		{"abc123.lambda-url.us-east-1.on.aws:443", urlID, true},
		{"abc123.lambda-url.on.aws", "", false},
		{"abc123.lambda-url.a.b.on.aws", "", false},
		{"app.lambda-url.example.com", "", false},
		{"a.b.lambda-url.localhost", "", false},
		{".lambda-url.localhost", "", false},
		{"lambda-url.localhost", "", false},
		{"[::1]:4566", "", false},
		{"", "", false},
	}

	for _, tc := range cases {
		id, ok := FunctionURLID(tc.host)
		if id != tc.id || ok != tc.ok {
			t.Errorf("FunctionURLID(%q) = %q, %v; want %q, %v", tc.host, id, ok, tc.id, tc.ok)
		}
	}
}

func TestExecuteAPIID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		host string
		id   string
		ok   bool
	}{
		{"api123.execute-api.localhost", apiID, true},
		{"api123.execute-api.localhost:4566", apiID, true},
		{"api123.execute-api.us-east-1.amazonaws.com", apiID, true},
		{"api.execute-api.example.com", "", false},
		{"a.b.execute-api.localhost", "", false},
		{".execute-api.localhost", "", false},
		{"execute-api.localhost", "", false},
		{"", "", false},
	}

	for _, tc := range cases {
		id, ok := ExecuteAPIID(tc.host)
		if id != tc.id || ok != tc.ok {
			t.Errorf("ExecuteAPIID(%q) = %q, %v; want %q, %v", tc.host, id, ok, tc.id, tc.ok)
		}
	}
}

func TestStripPort(t *testing.T) {
	t.Parallel()

	for host, want := range map[string]string{localhost + ":4566": localhost, localhost: localhost, ipv6Loopback + ":4566": ipv6Loopback, ipv6Loopback: ipv6Loopback} {
		if got := StripPort(host); got != want {
			t.Errorf("StripPort(%q) = %q, want %q", host, got, want)
		}
	}
}
