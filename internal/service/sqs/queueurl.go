package sqs

import (
	"net/http"
	"net/url"
)

const (
	schemeHTTP  = "http"
	schemeHTTPS = "https"
)

// requestQueueURL rewrites the scheme and host of a stored queue URL to the
// endpoint that served r, so SDKs that follow the returned QueueUrl reach kumo.
//
// The stored URL is returned unchanged when r.Host is empty or the stored
// URL cannot be parsed. Storage keys are never rewritten; queue lookups
// resolve by path, so any host the client sends back still works.
func requestQueueURL(r *http.Request, stored string) string {
	if r.Host == "" {
		return stored
	}

	u, err := url.Parse(stored)
	if err != nil {
		return stored
	}

	u.Scheme = schemeHTTP
	if r.TLS != nil {
		u.Scheme = schemeHTTPS
	}

	u.Host = r.Host

	return u.String()
}
