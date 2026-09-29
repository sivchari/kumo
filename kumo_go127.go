//go:build go1.27

package kumo

import (
	"net/http/httptest"
	"testing"
)

// NewTestServer creates a new in-process AWS emulator server that uses the
// httptest in-memory network instead of a loopback listener, which makes it
// usable from within a testing/synctest bubble. It requires Go 1.27 or later.
//
// The server is shut down automatically when the test ends. Requests must be
// sent with the client returned by httptest.Server.Client, since the server is
// not reachable over the loopback network. That client routes every request to
// the server regardless of the destination address, so the base endpoint can be
// any URL:
//
//	srv := kumo.NewTestServer(t)
//	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
//	    o.BaseEndpoint = aws.String("http://example.com")
//	    o.HTTPClient = srv.Client()
//	})
func NewTestServer(t testing.TB) *httptest.Server {
	t.Helper()
	return httptest.NewTestServer(t, handler())
}
