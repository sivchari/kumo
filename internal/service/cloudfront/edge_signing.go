package cloudfront

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha1" //nolint:gosec // CloudFront uses RSA-SHA1 for signed URL/cookie verification.
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	schemeHTTPS = "https"
	schemeHTTP  = "http"
)

// signedCredentials holds the three components needed to verify a
// CloudFront signed request (cookie or URL).
type signedCredentials struct {
	Policy    string // CF-Base64-encoded custom policy JSON, or "" for canned
	Signature string // CF-Base64-encoded RSA-SHA1 signature
	KeyPairID string // ID of the public key to use for verification
	Expires   int64  // Unix epoch for canned policy (from Expires param)
}

// cfPolicy is the JSON structure of a CloudFront access policy.
//
//nolint:tagliatelle // AWS CloudFront policy JSON uses PascalCase keys.
type cfPolicy struct {
	Statement []cfStatement `json:"Statement"`
}

//nolint:tagliatelle // AWS CloudFront policy JSON uses PascalCase keys.
type cfStatement struct {
	Resource  string      `json:"Resource"`
	Condition cfCondition `json:"Condition"`
}

//nolint:tagliatelle // AWS CloudFront policy JSON uses PascalCase keys.
type cfCondition struct {
	DateLessThan    *cfEpoch  `json:"DateLessThan,omitempty"`
	DateGreaterThan *cfEpoch  `json:"DateGreaterThan,omitempty"`
	IPAddress       *cfIPAddr `json:"IpAddress,omitempty"`
}

//nolint:tagliatelle // AWS CloudFront policy JSON uses PascalCase keys.
type cfEpoch struct {
	EpochTime int64 `json:"AWS:EpochTime"`
}

//nolint:tagliatelle // AWS CloudFront policy JSON uses PascalCase keys.
type cfIPAddr struct {
	SourceIP string `json:"AWS:SourceIp"`
}

// extractSignedCredentials tries to find signed credentials first in
// cookies, then in query parameters. Returns nil if no credentials
// are present.
func extractSignedCredentials(r *http.Request) *signedCredentials {
	if creds := extractFromCookies(r); creds != nil {
		return creds
	}

	return extractFromQuery(r)
}

func extractFromCookies(r *http.Request) *signedCredentials {
	sig := cookieValue(r, "CloudFront-Signature")
	kid := cookieValue(r, "CloudFront-Key-Pair-Id")

	if sig == "" || kid == "" {
		return nil
	}

	return &signedCredentials{
		Policy:    cookieValue(r, "CloudFront-Policy"),
		Signature: sig,
		KeyPairID: kid,
	}
}

func extractFromQuery(r *http.Request) *signedCredentials {
	q := r.URL.Query()
	sig := q.Get("Signature")
	kid := q.Get("Key-Pair-Id")

	if sig == "" || kid == "" {
		return nil
	}

	creds := &signedCredentials{
		Signature: sig,
		KeyPairID: kid,
	}

	if policy := q.Get("Policy"); policy != "" {
		creds.Policy = policy
	} else if expires := q.Get("Expires"); expires != "" {
		exp, err := strconv.ParseInt(expires, 10, 64)
		if err == nil {
			creds.Expires = exp
		}
	}

	return creds
}

func cookieValue(r *http.Request, name string) string {
	c, err := r.Cookie(name)
	if err != nil {
		return ""
	}

	return c.Value
}

// checkEdgeSigning enforces signed cookie / signed URL verification
// when the distribution's DefaultCacheBehavior has TrustedKeyGroups
// enabled. Returns true when the request may proceed (either because
// signing is not required or the credentials are valid), false when an
// error response has already been written to w.
func (s *Service) checkEdgeSigning(w http.ResponseWriter, r *http.Request, dist *Distribution) bool {
	if !requiresSigning(dist) {
		return true
	}

	creds := extractSignedCredentials(r)
	if creds == nil {
		http.Error(w, "missing CloudFront signed credentials", http.StatusForbidden)

		return false
	}

	if err := s.verifySigned(r, dist, creds, time.Now()); err != nil {
		http.Error(w, "access denied: "+err.Error(), http.StatusForbidden)

		return false
	}

	return true
}

// verifySigned checks the signed credentials against the distribution's
// trusted key groups. Returns nil on success, or an error describing
// the failure.
func (s *Service) verifySigned(r *http.Request, dist *Distribution, creds *signedCredentials, now time.Time) error {
	pubKey, err := s.resolvePublicKey(r, dist, creds.KeyPairID)
	if err != nil {
		return err
	}

	policyBytes, err := policyDocument(creds, r)
	if err != nil {
		return fmt.Errorf("invalid policy: %w", err)
	}

	if err := verifyRSASHA1(pubKey, policyBytes, creds.Signature); err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	var policy cfPolicy
	if err := json.Unmarshal(policyBytes, &policy); err != nil {
		return fmt.Errorf("policy parse failed: %w", err)
	}

	return evaluatePolicy(&policy, r, now)
}

// resolvePublicKey finds the PEM-encoded public key matching keyPairID
// through the distribution's TrustedKeyGroups.
func (s *Service) resolvePublicKey(r *http.Request, dist *Distribution, keyPairID string) (*rsa.PublicKey, error) {
	dcb := dist.DistributionConfig.DefaultCacheBehavior
	if dcb == nil || dcb.TrustedKeyGroups == nil || !dcb.TrustedKeyGroups.Enabled {
		return nil, fmt.Errorf("distribution has no trusted key groups")
	}

	for _, kgID := range dcb.TrustedKeyGroups.Items {
		kg, err := s.storage.GetKeyGroup(r.Context(), kgID)
		if err != nil {
			continue
		}

		for _, pkID := range kg.KeyGroupConfig.Items {
			if pkID != keyPairID {
				continue
			}

			pk, err := s.storage.GetPublicKey(r.Context(), pkID)
			if err != nil {
				continue
			}

			return parseRSAPublicKey(pk.PublicKeyConfig.EncodedKey)
		}
	}

	return nil, fmt.Errorf("key pair %s not found in trusted key groups", keyPairID)
}

// parseRSAPublicKey parses a PEM-encoded RSA public key.
func parseRSAPublicKey(encoded string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(encoded))
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}

	return rsaPub, nil
}

// policyDocument returns the JSON bytes that the signature covers.
// For custom policies, it decodes the CF-Base64-encoded Policy field.
// For canned policies, it constructs the canonical JSON from the URL
// and Expires.
func policyDocument(creds *signedCredentials, r *http.Request) ([]byte, error) {
	if creds.Policy != "" {
		return cfBase64Decode(creds.Policy)
	}

	if creds.Expires == 0 {
		return nil, fmt.Errorf("no policy or expires provided")
	}

	resource := requestResourceURL(r)

	// Canned policy JSON must be compact with no whitespace — the
	// signer produces this exact layout and signs it.
	//nolint:gocritic // Canned policy JSON must match the exact format the signer produced; %q escapes differ from JSON.
	policy := fmt.Sprintf(
		`{"Statement":[{"Resource":"%s","Condition":{"DateLessThan":{"AWS:EpochTime":%d}}}]}`,
		resource, creds.Expires,
	)

	return []byte(policy), nil
}

// requestResourceURL reconstructs the resource URL for canned policy
// verification. Strips signing query parameters (Expires, Signature,
// Key-Pair-Id) so the base URL matches what the signer produced.
func requestResourceURL(r *http.Request) string {
	scheme := schemeHTTPS
	if r.TLS == nil {
		scheme = schemeHTTP
	}

	base := scheme + "://" + r.Host + r.URL.Path

	// Preserve non-signing query params.
	q := r.URL.Query()
	q.Del("Expires")
	q.Del("Signature")
	q.Del("Key-Pair-Id")
	q.Del("Policy")

	if encoded := q.Encode(); encoded != "" {
		base += "?" + encoded
	}

	return base
}

// cfBase64Decode decodes CloudFront's modified Base64 encoding.
// CloudFront replaces: + -> -, = -> _, / -> ~.
func cfBase64Decode(s string) ([]byte, error) {
	s = strings.ReplaceAll(s, "-", "+")
	s = strings.ReplaceAll(s, "_", "=")
	s = strings.ReplaceAll(s, "~", "/")

	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}

	return decoded, nil
}

// verifyRSASHA1 verifies an RSA-SHA1 signature against the given
// message using CloudFront's modified Base64 for the signature.
func verifyRSASHA1(pub *rsa.PublicKey, message []byte, sig string) error {
	sigBytes, err := cfBase64Decode(sig)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}

	h := sha1.Sum(message) //nolint:gosec // CloudFront protocol mandates SHA1.

	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA1, h[:], sigBytes); err != nil {
		return fmt.Errorf("rsa verify: %w", err)
	}

	return nil
}

// evaluatePolicy checks the policy conditions against the current
// request context (time, IP).
func evaluatePolicy(policy *cfPolicy, r *http.Request, now time.Time) error {
	if len(policy.Statement) == 0 {
		return fmt.Errorf("policy has no statements")
	}

	stmt := policy.Statement[0]
	cond := stmt.Condition

	if cond.DateLessThan != nil {
		if now.Unix() > cond.DateLessThan.EpochTime {
			return fmt.Errorf("access expired at %d", cond.DateLessThan.EpochTime)
		}
	}

	if cond.DateGreaterThan != nil {
		if now.Unix() < cond.DateGreaterThan.EpochTime {
			return fmt.Errorf("access not yet valid until %d", cond.DateGreaterThan.EpochTime)
		}
	}

	if cond.IPAddress != nil && cond.IPAddress.SourceIP != "" {
		if err := checkIPCondition(r, cond.IPAddress.SourceIP); err != nil {
			return err
		}
	}

	return nil
}

// checkIPCondition verifies the client IP is within the allowed range.
func checkIPCondition(r *http.Request, allowedCIDR string) error {
	clientIP := extractClientIP(r)
	if clientIP == "" {
		return fmt.Errorf("cannot determine client IP")
	}

	_, ipNet, err := net.ParseCIDR(allowedCIDR)
	if err != nil {
		// Not a CIDR — try as bare IP (append /32 or /128).
		ip := net.ParseIP(allowedCIDR)
		if ip == nil {
			return fmt.Errorf("invalid IP condition: %s", allowedCIDR)
		}

		if clientIP != ip.String() {
			return fmt.Errorf("client IP %s not allowed", clientIP)
		}

		return nil
	}

	client := net.ParseIP(clientIP)
	if client == nil || !ipNet.Contains(client) {
		return fmt.Errorf("client IP %s not in allowed range %s", clientIP, allowedCIDR)
	}

	return nil
}

// extractClientIP returns the client's IP from RemoteAddr, stripping
// the port if present.
func extractClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}

// requiresSigning reports whether the distribution requires signed
// access (via TrustedKeyGroups on the DefaultCacheBehavior).
func requiresSigning(dist *Distribution) bool {
	if dist == nil || dist.DistributionConfig == nil {
		return false
	}

	dcb := dist.DistributionConfig.DefaultCacheBehavior
	if dcb == nil {
		return false
	}

	return dcb.TrustedKeyGroups != nil && dcb.TrustedKeyGroups.Enabled
}
