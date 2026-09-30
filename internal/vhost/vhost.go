// Package vhost recognises the virtual-hosted AWS endpoints kumo serves by
// Host header. The router dispatches requests with these hosts, and the
// CloudFront edge uses the same predicates to decide which custom origins it
// can reach through kumo's own listener instead of the network.
package vhost

import "strings"

const localhost = "localhost"

// FunctionURLID recognises Lambda function URL hosts and returns the
// lower-cased url id. Recognised shapes (the kumo-local form resolves to
// loopback):
//
//	{urlId}.lambda-url.localhost(:port)
//	{urlId}.lambda-url.{region}.on.aws
func FunctionURLID(host string) (string, bool) {
	host = strings.ToLower(StripPort(host))

	urlID, rest, found := strings.Cut(host, ".lambda-url.")
	if !found || urlID == "" || strings.Contains(urlID, ".") {
		return "", false
	}

	if rest == localhost {
		return urlID, true
	}

	region, domain, ok := strings.Cut(rest, ".")
	if ok && region != "" && !strings.Contains(region, ".") && domain == "on.aws" {
		return urlID, true
	}

	return "", false
}

// ExecuteAPIID recognises API Gateway execute-api hosts and returns the API
// id. Recognised shapes:
//
//	{apiId}.execute-api.localhost(:port)
//	{apiId}.execute-api.{region}.amazonaws.com
func ExecuteAPIID(host string) (string, bool) {
	host = StripPort(host)

	apiID, rest, found := strings.Cut(host, ".execute-api.")
	if !found || apiID == "" || strings.Contains(apiID, ".") {
		return "", false
	}

	if rest == localhost || strings.HasSuffix(rest, ".amazonaws.com") {
		return apiID, true
	}

	return "", false
}

// StripPort removes an optional :port from a Host header value, leaving
// bracketed IPv6 literals intact.
func StripPort(host string) string {
	if strings.HasSuffix(host, "]") {
		return host
	}

	if idx := strings.LastIndex(host, ":"); idx >= 0 && !strings.Contains(host[idx:], "]") {
		return host[:idx]
	}

	return host
}
