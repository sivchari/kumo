package sqs

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

const storedTestQueueURL = "http://localhost:4566/000000000000/test-queue"

type requestQueueURLCase struct {
	name   string
	host   string
	tls    bool
	stored string
	want   string
}

func requestQueueURLCases() []requestQueueURLCase {
	return []requestQueueURLCase{
		{
			name:   "same host",
			host:   "localhost:4566",
			stored: storedTestQueueURL,
			want:   "http://localhost:4566/000000000000/test-queue",
		},
		{
			name:   "different port",
			host:   "localhost:14566",
			stored: storedTestQueueURL,
			want:   "http://localhost:14566/000000000000/test-queue",
		},
		{
			name:   "different hostname",
			host:   "kumo:4566",
			stored: storedTestQueueURL,
			want:   "http://kumo:4566/000000000000/test-queue",
		},
		{
			name:   "tls request uses https",
			host:   "kumo.example.com:4566",
			tls:    true,
			stored: storedTestQueueURL,
			want:   "https://kumo.example.com:4566/000000000000/test-queue",
		},
		{
			name:   "empty host keeps stored",
			host:   "",
			stored: storedTestQueueURL,
			want:   storedTestQueueURL,
		},
		{
			name:   "ipv6 host",
			host:   "[::1]:4566",
			stored: storedTestQueueURL,
			want:   "http://[::1]:4566/000000000000/test-queue",
		},
		{
			name:   "unparsable stored keeps stored",
			host:   "localhost:14566",
			stored: "http://[::1/000000000000/test-queue",
			want:   "http://[::1/000000000000/test-queue",
		},
	}
}

func TestRequestQueueURL(t *testing.T) {
	t.Parallel()

	for _, tt := range requestQueueURLCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", http.NoBody)
			r.Host = tt.host

			if tt.tls {
				r.TLS = &tls.ConnectionState{}
			}

			if got := requestQueueURL(r, tt.stored); got != tt.want {
				t.Errorf("requestQueueURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
