package s3

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// WebsiteConfiguration is the XML body of PUT/GET /{bucket}?website,
// matching S3's `WebsiteConfiguration` schema.
//
// kumo doesn't actually serve the bucket as a website (no redirect
// dispatch, no error-document substitution at request time) — it just
// roundtrips the configuration so terraform / cdk / pulumi resources
// like `aws_s3_bucket_website_configuration` see consistent state.
type WebsiteConfiguration struct {
	XMLName               xml.Name               `xml:"WebsiteConfiguration"`
	Xmlns                 string                 `xml:"xmlns,attr,omitempty"`
	IndexDocument         *WebsiteIndexDocument  `xml:"IndexDocument,omitempty"`
	ErrorDocument         *WebsiteErrorDocument  `xml:"ErrorDocument,omitempty"`
	RedirectAllRequestsTo *RedirectAllRequestsTo `xml:"RedirectAllRequestsTo,omitempty"`
	RoutingRules          *WebsiteRoutingRules   `xml:"RoutingRules,omitempty"`
}

// WebsiteIndexDocument names the file served for `/` requests.
type WebsiteIndexDocument struct {
	Suffix string `xml:"Suffix"`
}

// WebsiteErrorDocument names the file served on 4xx.
type WebsiteErrorDocument struct {
	Key string `xml:"Key"`
}

// RedirectAllRequestsTo unconditionally redirects every request.
type RedirectAllRequestsTo struct {
	HostName string `xml:"HostName"`
	Protocol string `xml:"Protocol,omitempty"`
}

// WebsiteRoutingRules wraps the list of routing rules.
type WebsiteRoutingRules struct {
	Rules []WebsiteRoutingRule `xml:"RoutingRule"`
}

// WebsiteRoutingRule is one conditional redirect rule.
type WebsiteRoutingRule struct {
	Condition *RoutingRuleCondition `xml:"Condition,omitempty"`
	Redirect  RoutingRuleRedirect   `xml:"Redirect"`
}

// RoutingRuleCondition is the condition that triggers a routing rule.
type RoutingRuleCondition struct {
	KeyPrefixEquals             string `xml:"KeyPrefixEquals,omitempty"`
	HTTPErrorCodeReturnedEquals string `xml:"HttpErrorCodeReturnedEquals,omitempty"`
}

// RoutingRuleRedirect is the redirect target of a routing rule.
type RoutingRuleRedirect struct {
	HostName             string `xml:"HostName,omitempty"`
	Protocol             string `xml:"Protocol,omitempty"`
	ReplaceKeyPrefixWith string `xml:"ReplaceKeyPrefixWith,omitempty"`
	ReplaceKeyWith       string `xml:"ReplaceKeyWith,omitempty"`
	HTTPRedirectCode     string `xml:"HttpRedirectCode,omitempty"`
}

// PutBucketWebsite handles PUT /{bucket}?website.
func (s *Service) PutBucketWebsite(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeS3Error(w, r, "InvalidRequest", "Failed to read request body", http.StatusBadRequest)

		return
	}

	var cfg WebsiteConfiguration
	if err := xml.Unmarshal(body, &cfg); err != nil {
		writeS3Error(w, r, "MalformedXML", fmt.Sprintf("WebsiteConfiguration XML: %v", err), http.StatusBadRequest)

		return
	}

	if err := s.storage.PutBucketWebsite(r.Context(), bucket, &cfg); err != nil {
		handleBucketLevelError(w, r, err)

		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetBucketWebsite handles GET /{bucket}?website.
func (s *Service) GetBucketWebsite(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")

	cfg, err := s.storage.GetBucketWebsite(r.Context(), bucket)
	if err != nil {
		handleBucketLevelError(w, r, err)

		return
	}

	cfg.Xmlns = s3Namespace
	writeXMLResponse(w, cfg)
}

// DeleteBucketWebsite handles DELETE /{bucket}?website.
func (s *Service) DeleteBucketWebsite(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")

	if err := s.storage.DeleteBucketWebsite(r.Context(), bucket); err != nil {
		handleBucketLevelError(w, r, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// MemoryStorage hooks for bucket website configuration.

// PutBucketWebsite stores the website configuration on the bucket.
func (s *MemoryStorage) PutBucketWebsite(_ context.Context, bucket string, cfg *WebsiteConfiguration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.Buckets[bucket]
	if !ok {
		return &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	b.Website = cfg

	return nil
}

// GetBucketWebsite returns the stored website configuration, or
// NoSuchWebsiteConfiguration if none has been set.
func (s *MemoryStorage) GetBucketWebsite(_ context.Context, bucket string) (*WebsiteConfiguration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	b, ok := s.Buckets[bucket]
	if !ok {
		return nil, &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	if b.Website == nil {
		return nil, &BucketError{Code: "NoSuchWebsiteConfiguration", Message: "The specified bucket does not have a website configuration", BucketName: bucket}
	}

	return b.Website, nil
}

// DeleteBucketWebsite removes the website configuration from the
// bucket. Idempotent — DELETE on an unconfigured bucket is a no-op.
func (s *MemoryStorage) DeleteBucketWebsite(_ context.Context, bucket string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.Buckets[bucket]
	if !ok {
		return &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	b.Website = nil

	return nil
}

// handleBucketLevelError maps BucketError → status code. Used by the
// website / lifecycle / restore handlers added in this PR.
func handleBucketLevelError(w http.ResponseWriter, r *http.Request, err error) {
	var bucketErr *BucketError
	if errors.As(err, &bucketErr) {
		status := http.StatusNotFound

		writeS3Error(w, r, bucketErr.Code, bucketErr.Message, status)

		return
	}

	var objErr *ObjectError
	if errors.As(err, &objErr) {
		writeS3Error(w, r, objErr.Code, objErr.Message, http.StatusNotFound)

		return
	}

	writeS3Error(w, r, "InternalError", "Internal server error", http.StatusInternalServerError)
}
