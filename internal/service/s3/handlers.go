package s3

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	xmlHeader      = `<?xml version="1.0" encoding="UTF-8"?>`
	s3Namespace    = "http://s3.amazonaws.com/doc/2006-03-01/"
	timeFormatISO  = "2006-01-02T15:04:05.000Z"
	timeFormatHTTP = "Mon, 02 Jan 2006 15:04:05 GMT"

	// contentTypeHeader is the canonical "Content-Type" string. Hoisted
	// to a const because the metadata-pass loop excludes it in three
	// different handlers — goconst was flagging the literal.
	contentTypeHeader = "Content-Type"
)

// applyCORSHeaders sets CORS response headers if the bucket has CORS configured and the request Origin matches.
func (s *Service) applyCORSHeaders(w http.ResponseWriter, r *http.Request, bucket string) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return
	}

	rules := s.storage.GetCORSRules(r.Context(), bucket)
	if len(rules) == 0 {
		return
	}

	for _, rule := range rules {
		if !matchOrigin(origin, rule.AllowedOrigins) {
			continue
		}

		if !matchMethod(r.Method, rule.AllowedMethods) {
			continue
		}

		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", strings.Join(rule.AllowedMethods, ", "))

		if len(rule.AllowedHeaders) > 0 {
			w.Header().Set("Access-Control-Allow-Headers", strings.Join(rule.AllowedHeaders, ", "))
		}

		if len(rule.ExposeHeaders) > 0 {
			w.Header().Set("Access-Control-Expose-Headers", strings.Join(rule.ExposeHeaders, ", "))
		}

		if rule.MaxAgeSeconds > 0 {
			w.Header().Set("Access-Control-Max-Age", strconv.Itoa(rule.MaxAgeSeconds))
		}

		return
	}
}

func matchOrigin(origin string, allowed []string) bool {
	for _, a := range allowed {
		if a == "*" || a == origin {
			return true
		}
	}

	return false
}

func matchMethod(method string, allowed []string) bool {
	for _, a := range allowed {
		if strings.EqualFold(a, method) {
			return true
		}
	}

	return false
}

// HandleCORSPreflight handles OPTIONS requests for CORS preflight.
// For preflight, the actual method is in Access-Control-Request-Method header.
func (s *Service) HandleCORSPreflight(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")

	// Override method for CORS matching: use the requested method from the preflight header.
	if reqMethod := r.Header.Get("Access-Control-Request-Method"); reqMethod != "" {
		r.Method = reqMethod
	}

	s.applyCORSHeaders(w, r, bucket)
	w.WriteHeader(http.StatusOK)
}

// Route Dispatchers - dispatch based on query parameters

// handleBucketGet dispatches GET /{bucket} requests based on query parameters.
func (s *Service) handleBucketGet(w http.ResponseWriter, r *http.Request) {
	if _, ok := r.URL.Query()["versioning"]; ok {
		s.GetBucketVersioning(w, r)

		return
	}

	if _, ok := r.URL.Query()["publicAccessBlock"]; ok {
		s.GetPublicAccessBlock(w, r)

		return
	}

	if _, ok := r.URL.Query()["encryption"]; ok {
		s.GetBucketEncryption(w, r)

		return
	}

	if _, ok := r.URL.Query()["policy"]; ok {
		s.GetBucketPolicy(w, r)

		return
	}

	if _, ok := r.URL.Query()["logging"]; ok {
		s.GetBucketLogging(w, r)

		return
	}

	if _, ok := r.URL.Query()["versions"]; ok {
		s.ListObjectVersions(w, r)

		return
	}

	if _, ok := r.URL.Query()["uploads"]; ok {
		s.ListMultipartUploads(w, r)

		return
	}

	if handled := s.serveBucketSubresourceStub(w, r); handled {
		return
	}

	if r.URL.Query().Get("list-type") == "2" {
		s.ListObjects(w, r)

		return
	}

	s.ListObjectsV1(w, r)
}

// serveBucketSubresourceStub handles GET requests for bucket sub-resources that
// kumo does not model. Some sub-resources (acl, location, logging, accelerate,
// requestPayment) always return a default response in real S3; others return a
// specific NoSuch* error code. Returns true when the request was handled.
func (s *Service) serveBucketSubresourceStub(w http.ResponseWriter, r *http.Request) bool {
	q := r.URL.Query()

	switch {
	case q.Has("acl"):
		writeXMLResponse(w, defaultBucketACL())
	case q.Has("location"):
		writeXMLResponse(w, struct {
			XMLName xml.Name `xml:"LocationConstraint"`
			Xmlns   string   `xml:"xmlns,attr,omitempty"`
			Value   string   `xml:",chardata"`
		}{Xmlns: s3Namespace, Value: "us-east-1"})
	case q.Has("accelerate"):
		writeXMLResponse(w, struct {
			XMLName xml.Name `xml:"AccelerateConfiguration"`
			Xmlns   string   `xml:"xmlns,attr,omitempty"`
			Status  string   `xml:"Status,omitempty"`
		}{Xmlns: s3Namespace})
	case q.Has("requestPayment"):
		writeXMLResponse(w, struct {
			XMLName xml.Name `xml:"RequestPaymentConfiguration"`
			Xmlns   string   `xml:"xmlns,attr,omitempty"`
			Payer   string   `xml:"Payer"`
		}{Xmlns: s3Namespace, Payer: "BucketOwner"})
	default:
		errCode, ok := bucketSubresourceErrorCode(q)
		if !ok {
			return false
		}

		writeS3Error(w, r, errCode, "The "+errCode+" sub-resource is not configured", http.StatusNotFound)
	}

	return true
}

// bucketSubresourceErrorCode maps a GET ?<sub-resource> query to the AWS error
// code returned when the sub-resource is unconfigured.
func bucketSubresourceErrorCode(q map[string][]string) (string, bool) {
	mapping := map[string]string{
		"cors":              "NoSuchCORSConfiguration",
		"lifecycle":         "NoSuchLifecycleConfiguration",
		"replication":       "ReplicationConfigurationNotFoundError",
		"website":           "NoSuchWebsiteConfiguration",
		"tagging":           "NoSuchTagSet",
		"object-lock":       "ObjectLockConfigurationNotFoundError",
		"ownershipControls": "OwnershipControlsNotFoundError",
	}

	for key, code := range mapping {
		if _, ok := q[key]; ok {
			return code, true
		}
	}

	return "", false
}

// defaultBucketACL builds the ACL response real S3 returns when the bucket has
// no explicit ACL set: owner gets FULL_CONTROL.
func defaultBucketACL() any {
	type grantee struct {
		XMLName     xml.Name `xml:"Grantee"`
		XSI         string   `xml:"xmlns:xsi,attr"`
		Type        string   `xml:"xsi:type,attr"`
		ID          string   `xml:"ID,omitempty"`
		DisplayName string   `xml:"DisplayName,omitempty"`
	}

	type grant struct {
		XMLName    xml.Name `xml:"Grant"`
		Grantee    grantee
		Permission string `xml:"Permission"`
	}

	type owner struct {
		XMLName     xml.Name `xml:"Owner"`
		ID          string   `xml:"ID"`
		DisplayName string   `xml:"DisplayName,omitempty"`
	}

	return struct {
		XMLName           xml.Name `xml:"AccessControlPolicy"`
		Xmlns             string   `xml:"xmlns,attr,omitempty"`
		Owner             owner
		AccessControlList struct {
			Grant grant
		} `xml:"AccessControlList"`
	}{
		Xmlns: s3Namespace,
		Owner: owner{ID: "owner-id", DisplayName: "owner"},
		AccessControlList: struct {
			Grant grant
		}{
			Grant: grant{
				Grantee: grantee{
					XSI:         "http://www.w3.org/2001/XMLSchema-instance",
					Type:        "CanonicalUser",
					ID:          "owner-id",
					DisplayName: "owner",
				},
				Permission: "FULL_CONTROL",
			},
		},
	}
}

// handleBucketPost dispatches POST /{bucket} requests based on query parameters.
func (s *Service) handleBucketPost(w http.ResponseWriter, r *http.Request) {
	if _, ok := r.URL.Query()["delete"]; ok {
		s.DeleteObjects(w, r)

		return
	}

	writeS3Error(w, r, "InvalidRequest", "Invalid request", http.StatusBadRequest)
}

// handleObjectPut dispatches PUT /{bucket}/{key} requests based on query parameters.
func (s *Service) handleObjectPut(w http.ResponseWriter, r *http.Request) {
	s.applyCORSHeaders(w, r, r.PathValue("bucket"))

	if r.URL.Query().Has("acl") {
		s.PutObjectACL(w, r)

		return
	}

	if r.URL.Query().Has("tagging") {
		s.PutObjectTagging(w, r)

		return
	}

	if r.URL.Query().Get("uploadId") != "" && r.URL.Query().Get("partNumber") != "" {
		if r.Header.Get("X-Amz-Copy-Source") != "" {
			s.UploadPartCopy(w, r)

			return
		}

		s.UploadPart(w, r)

		return
	}

	if r.Header.Get("X-Amz-Copy-Source") != "" {
		s.CopyObject(w, r)

		return
	}

	s.PutObject(w, r)
}

// handleObjectGet dispatches GET /{bucket}/{key} requests based on query parameters.
func (s *Service) handleObjectGet(w http.ResponseWriter, r *http.Request) {
	s.applyCORSHeaders(w, r, r.PathValue("bucket"))

	if r.URL.Query().Has("acl") {
		s.GetObjectACL(w, r)

		return
	}

	if r.URL.Query().Has("tagging") {
		s.GetObjectTagging(w, r)

		return
	}

	if r.URL.Query().Get("uploadId") != "" {
		s.ListParts(w, r)

		return
	}

	s.GetObject(w, r)
}

// handleObjectDelete dispatches DELETE /{bucket}/{key} requests based on query parameters.
func (s *Service) handleObjectDelete(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("uploadId") != "" {
		s.AbortMultipartUpload(w, r)

		return
	}

	s.DeleteObject(w, r)
}

// handleObjectPost dispatches POST /{bucket}/{key} requests based on query parameters.
func (s *Service) handleObjectPost(w http.ResponseWriter, r *http.Request) {
	if _, ok := r.URL.Query()["uploads"]; ok {
		s.CreateMultipartUpload(w, r)

		return
	}

	if r.URL.Query().Get("uploadId") != "" {
		s.CompleteMultipartUpload(w, r)

		return
	}

	writeS3Error(w, r, "InvalidRequest", "Invalid request", http.StatusBadRequest)
}

// ListBuckets handles GET / - list all buckets.
func (s *Service) ListBuckets(w http.ResponseWriter, r *http.Request) {
	buckets, err := s.storage.ListBuckets(r.Context())
	if err != nil {
		writeS3Error(w, r, "InternalError", "Internal server error", http.StatusInternalServerError)

		return
	}

	bucketInfos := make([]BucketInfo, len(buckets))

	for i, b := range buckets {
		bucketInfos[i] = BucketInfo{
			Name:         b.Name,
			CreationDate: b.CreationDate.Format(timeFormatISO),
			BucketArn:    "arn:aws:s3:::" + b.Name,
		}
	}

	result := ListAllMyBucketsResult{
		Xmlns: s3Namespace,
		Buckets: Buckets{
			Bucket: bucketInfos,
		},
		Owner: Owner{
			ID: "owner-id",
		},
	}

	writeXMLResponse(w, result)
}

// CreateBucket handles PUT /{bucket} - create a bucket.
func (s *Service) CreateBucket(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	if bucket == "" {
		writeS3Error(w, r, "InvalidBucketName", "The specified bucket is not valid.", http.StatusBadRequest)

		return
	}

	err := s.storage.CreateBucket(r.Context(), bucket)
	if err != nil {
		var bucketErr *BucketError
		if errors.As(err, &bucketErr) {
			switch bucketErr.Code {
			case "BucketAlreadyOwnedByYou":
				writeS3Error(w, r, bucketErr.Code, bucketErr.Message, http.StatusConflict)
			default:
				writeS3Error(w, r, bucketErr.Code, bucketErr.Message, http.StatusBadRequest)
			}

			return
		}

		writeS3Error(w, r, "InternalError", "Internal server error", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Location", "/"+bucket)
	w.WriteHeader(http.StatusOK)
}

// DeleteBucket handles DELETE /{bucket} - delete a bucket.
func (s *Service) DeleteBucket(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	if bucket == "" {
		writeS3Error(w, r, "InvalidBucketName", "The specified bucket is not valid.", http.StatusBadRequest)

		return
	}

	err := s.storage.DeleteBucket(r.Context(), bucket)
	if err != nil {
		var bucketErr *BucketError
		if errors.As(err, &bucketErr) {
			switch bucketErr.Code {
			case "NoSuchBucket":
				writeS3Error(w, r, bucketErr.Code, bucketErr.Message, http.StatusNotFound)
			case "BucketNotEmpty":
				writeS3Error(w, r, bucketErr.Code, bucketErr.Message, http.StatusConflict)
			default:
				writeS3Error(w, r, bucketErr.Code, bucketErr.Message, http.StatusBadRequest)
			}

			return
		}

		writeS3Error(w, r, "InternalError", "Internal server error", http.StatusInternalServerError)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HeadBucket handles HEAD /{bucket} - check bucket existence.
func (s *Service) HeadBucket(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	if bucket == "" {
		w.WriteHeader(http.StatusBadRequest)

		return
	}

	exists, err := s.storage.BucketExists(r.Context(), bucket)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)

		return
	}

	if !exists {
		w.WriteHeader(http.StatusNotFound)

		return
	}

	w.WriteHeader(http.StatusOK)
}

// ListObjects handles GET /{bucket} - list objects in a bucket.
func (s *Service) ListObjects(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	if bucket == "" {
		writeS3Error(w, r, "InvalidBucketName", "The specified bucket is not valid.", http.StatusBadRequest)

		return
	}

	prefix := r.URL.Query().Get("prefix")
	delimiter := r.URL.Query().Get("delimiter")
	maxKeys := 1000

	if maxKeysStr := r.URL.Query().Get("max-keys"); maxKeysStr != "" {
		if mk, err := strconv.Atoi(maxKeysStr); err == nil && mk > 0 {
			maxKeys = mk
		}
	}

	objects, commonPrefixes, err := s.storage.ListObjects(r.Context(), bucket, prefix, delimiter, maxKeys)
	if err != nil {
		var bucketErr *BucketError
		if errors.As(err, &bucketErr) {
			writeS3Error(w, r, bucketErr.Code, bucketErr.Message, http.StatusNotFound)

			return
		}

		writeS3Error(w, r, "InternalError", "Internal server error", http.StatusInternalServerError)

		return
	}

	contents := make([]ObjectInfo, len(objects))
	for i := range objects {
		contents[i] = ObjectInfo{
			Key:          objects[i].Key,
			LastModified: objects[i].LastModified.Format(timeFormatISO),
			ETag:         objects[i].ETag,
			Size:         objects[i].Size,
			StorageClass: "STANDARD",
		}
	}

	prefixes := make([]CommonPrefix, len(commonPrefixes))
	for i, p := range commonPrefixes {
		prefixes[i] = CommonPrefix{Prefix: p}
	}

	result := ListBucketResult{
		Xmlns:          s3Namespace,
		Name:           bucket,
		Prefix:         prefix,
		KeyCount:       len(objects),
		MaxKeys:        maxKeys,
		IsTruncated:    false,
		Contents:       contents,
		CommonPrefixes: prefixes,
	}

	writeXMLResponse(w, result)
}

// ListObjectsV1 handles GET /{bucket} (no list-type=2) — the legacy
// marker-based listing API. SDK v1, awscli's `aws s3 ls`, and a
// handful of non-AWS S3 clients still target it.
func (s *Service) ListObjectsV1(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	if bucket == "" {
		writeS3Error(w, r, "InvalidBucketName", "The specified bucket is not valid.", http.StatusBadRequest)

		return
	}

	q := r.URL.Query()
	params := parseListObjectsV1Params(q)

	const fetchAll = 1 << 30 // sentinel: "give us everything, we paginate in the handler"

	objects, commonPrefixes, err := s.storage.ListObjects(r.Context(), bucket, params.prefix, params.delimiter, fetchAll)
	if err != nil {
		writeListObjectsV1Error(w, r, err)

		return
	}

	objects = sliceObjectsAfterMarker(objects, params.marker)

	truncated := len(objects) > params.maxKeys
	if truncated {
		objects = objects[:params.maxKeys]
	}

	writeXMLResponse(w, buildListBucketResultV1(bucket, params, objects, commonPrefixes, truncated))
}

// listObjectsV1Params bundles the parsed query-string for a V1 list.
type listObjectsV1Params struct {
	prefix    string
	delimiter string
	marker    string
	maxKeys   int
}

func parseListObjectsV1Params(q map[string][]string) listObjectsV1Params {
	maxKeys := 1000

	if mks := firstQueryValue(q, "max-keys"); mks != "" {
		if mk, err := strconv.Atoi(mks); err == nil && mk > 0 {
			maxKeys = mk
		}
	}

	return listObjectsV1Params{
		prefix:    firstQueryValue(q, "prefix"),
		delimiter: firstQueryValue(q, "delimiter"),
		marker:    firstQueryValue(q, "marker"),
		maxKeys:   maxKeys,
	}
}

func writeListObjectsV1Error(w http.ResponseWriter, r *http.Request, err error) {
	var bucketErr *BucketError
	if errors.As(err, &bucketErr) {
		writeS3Error(w, r, bucketErr.Code, bucketErr.Message, http.StatusNotFound)

		return
	}

	writeS3Error(w, r, "InternalError", "Internal server error", http.StatusInternalServerError)
}

func buildListBucketResultV1(bucket string, params listObjectsV1Params, objects []Object, commonPrefixes []string, truncated bool) ListBucketResultV1 {
	contents := make([]ObjectInfo, len(objects))
	for i := range objects {
		contents[i] = ObjectInfo{
			Key:          objects[i].Key,
			LastModified: objects[i].LastModified.Format(timeFormatISO),
			ETag:         objects[i].ETag,
			Size:         objects[i].Size,
			StorageClass: "STANDARD",
		}
	}

	prefixes := make([]CommonPrefix, len(commonPrefixes))
	for i, p := range commonPrefixes {
		prefixes[i] = CommonPrefix{Prefix: p}
	}

	var nextMarker string
	if truncated && params.delimiter != "" && len(contents) > 0 {
		nextMarker = contents[len(contents)-1].Key
	}

	return ListBucketResultV1{
		Xmlns:          s3Namespace,
		Name:           bucket,
		Prefix:         params.prefix,
		Marker:         params.marker,
		NextMarker:     nextMarker,
		MaxKeys:        params.maxKeys,
		Delimiter:      params.delimiter,
		IsTruncated:    truncated,
		Contents:       contents,
		CommonPrefixes: prefixes,
	}
}

// sliceObjectsAfterMarker drops every entry whose Key is <= marker,
// matching the V1 spec ("listing starts after the marker key").
func sliceObjectsAfterMarker(objects []Object, marker string) []Object {
	if marker == "" {
		return objects
	}

	for i := range objects {
		if objects[i].Key > marker {
			return objects[i:]
		}
	}

	return nil
}

// PutObject handles PUT /{bucket}/{key...} - upload an object.
func (s *Service) PutObject(w http.ResponseWriter, r *http.Request) {
	if !checkPresignedURL(w, r) {
		return
	}

	bucket := r.PathValue("bucket")
	key := r.PathValue("key")

	if bucket == "" {
		writeS3Error(w, r, "InvalidBucketName", "The specified bucket is not valid.", http.StatusBadRequest)

		return
	}

	if key == "" {
		writeS3Error(w, r, "InvalidArgument", "Invalid key", http.StatusBadRequest)

		return
	}

	metadata := make(map[string]string)
	if ct := r.Header.Get("Content-Type"); ct != "" {
		metadata["Content-Type"] = ct
	}

	// Extract x-amz-meta-* headers
	for name, values := range r.Header {
		if metaKey, found := strings.CutPrefix(strings.ToLower(name), "x-amz-meta-"); found {
			metadata[metaKey] = values[0]
		}
	}

	obj, err := s.storage.PutObject(r.Context(), bucket, key, r.Body, metadata)
	if err != nil {
		var bucketErr *BucketError
		if errors.As(err, &bucketErr) {
			writeS3Error(w, r, bucketErr.Code, bucketErr.Message, http.StatusNotFound)

			return
		}

		writeS3Error(w, r, "InternalError", "Internal server error", http.StatusInternalServerError)

		return
	}

	w.Header().Set("ETag", obj.ETag)

	if obj.VersionID != "" {
		w.Header().Set("x-amz-version-id", obj.VersionID)
	}

	w.WriteHeader(http.StatusOK)

	// Emit EventBridge notification if enabled.
	go s.emitObjectCreatedEvent(context.Background(), bucket, key, obj.Size, obj.ETag)
}

// CopyObject handles PUT /{bucket}/{key} with X-Amz-Copy-Source header.
func (s *Service) CopyObject(w http.ResponseWriter, r *http.Request) {
	dstBucket := r.PathValue("bucket")
	dstKey := r.PathValue("key")

	copySource := r.Header.Get("X-Amz-Copy-Source")
	srcBucket, srcKey := parseCopySource(copySource)

	if srcBucket == "" || srcKey == "" {
		writeS3Error(w, r, "InvalidArgument", "Invalid copy source", http.StatusBadRequest)

		return
	}

	srcObj, err := s.storage.GetObject(r.Context(), srcBucket, srcKey)
	if err != nil {
		handleGetObjectError(w, r, err)

		return
	}

	if !evalCopySourcePreconditions(r.Header, srcObj.ETag, srcObj.LastModified) {
		writeS3Error(w, r, "PreconditionFailed", "At least one of the preconditions you specified did not hold.", http.StatusPreconditionFailed)

		return
	}

	dstObj, err := s.storage.PutObject(r.Context(), dstBucket, dstKey, bytes.NewReader(srcObj.Body), srcObj.Metadata)
	if err != nil {
		var bucketErr *BucketError
		if errors.As(err, &bucketErr) {
			writeS3Error(w, r, bucketErr.Code, bucketErr.Message, http.StatusNotFound)

			return
		}

		writeS3Error(w, r, "InternalError", "Internal server error", http.StatusInternalServerError)

		return
	}

	result := CopyObjectResult{
		ETag:         dstObj.ETag,
		LastModified: dstObj.LastModified.Format(timeFormatISO),
	}

	writeXMLResponse(w, result)

	go s.emitObjectCreatedEvent(context.Background(), dstBucket, dstKey, dstObj.Size, dstObj.ETag)
}

// parseCopySource parses the X-Amz-Copy-Source header value.
// Format: /bucket/key or bucket/key (URL-encoded).
func parseCopySource(source string) (bucket, key string) {
	source = strings.TrimPrefix(source, "/")

	idx := strings.IndexByte(source, '/')
	if idx < 0 {
		return "", ""
	}

	return source[:idx], source[idx+1:]
}

// GetObject handles GET /{bucket}/{key...} - download an object.
func (s *Service) GetObject(w http.ResponseWriter, r *http.Request) {
	if !checkPresignedURL(w, r) {
		return
	}

	bucket := r.PathValue("bucket")
	key := r.PathValue("key")

	if bucket == "" {
		writeS3Error(w, r, "InvalidBucketName", "The specified bucket is not valid.", http.StatusBadRequest)

		return
	}

	if key == "" {
		writeS3Error(w, r, "InvalidArgument", "Invalid key", http.StatusBadRequest)

		return
	}

	versionID := r.URL.Query().Get("versionId")

	var obj *Object

	var err error

	if versionID != "" {
		obj, err = s.storage.GetObjectVersion(r.Context(), bucket, key, versionID)
	} else {
		obj, err = s.storage.GetObject(r.Context(), bucket, key)
	}

	if err != nil {
		handleGetObjectError(w, r, err)

		return
	}

	switch evalGetObjectPreconditions(r.Header, obj.ETag, obj.LastModified) {
	case preconditionPass:
		// fall through to the normal response below
	case preconditionFailed:
		writeS3Error(w, r, "PreconditionFailed", "At least one of the preconditions you specified did not hold.", http.StatusPreconditionFailed)

		return
	case preconditionNotModified:
		writeNotModifiedResponse(w, obj)

		return
	}

	if rng := r.Header.Get("Range"); rng != "" {
		applyResponseHeaderOverrides(w, r.URL.Query())
		writeRangeOrFull(w, r, obj, rng)

		return
	}

	applyResponseHeaderOverrides(w, r.URL.Query())
	writeObjectResponse(w, obj)
}

// writeRangeOrFull serves a 206 Partial Content slice when the Range
// is satisfiable, a 416 Requested Range Not Satisfiable when the spec
// is well-formed but unsatisfiable, and falls through to a full 200
// when the unit isn't bytes (RFC 9110 §14.1.3 lets the server treat
// an unknown range unit as if the header were absent).
func writeRangeOrFull(w http.ResponseWriter, r *http.Request, obj *Object, rangeHeader string) {
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		writeObjectResponse(w, obj)

		return
	}

	start, end, ok := parseByteRange(rangeHeader, obj.Size)
	if !ok {
		w.Header().Set("Content-Range", "bytes */"+strconv.FormatInt(obj.Size, 10))
		writeS3Error(w, r, "InvalidRange",
			"The requested range is not satisfiable", http.StatusRequestedRangeNotSatisfiable)

		return
	}

	writePartialObjectResponse(w, obj, start, end)
}

// writePartialObjectResponse writes a 206 with the byte slice plus
// matching Content-Range / Content-Length / object metadata headers.
func writePartialObjectResponse(w http.ResponseWriter, obj *Object, start, end int64) {
	length := end - start + 1

	setIfAbsent(w, "Content-Type", obj.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	w.Header().Set("Content-Range",
		"bytes "+strconv.FormatInt(start, 10)+"-"+strconv.FormatInt(end, 10)+
			"/"+strconv.FormatInt(obj.Size, 10))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("ETag", obj.ETag)
	w.Header().Set("Last-Modified", obj.LastModified.UTC().Format(timeFormatHTTP))

	if obj.VersionID != "" {
		w.Header().Set("x-amz-version-id", obj.VersionID)
	}

	for k, v := range obj.Metadata {
		if k != contentTypeHeader {
			w.Header().Set("x-amz-meta-"+k, v)
		}
	}

	w.WriteHeader(http.StatusPartialContent)
	_, _ = w.Write(obj.Body[start : end+1])
}

// applyResponseHeaderOverrides honours the `response-*` query
// parameters S3 supports for presigned download URLs (mainly used to
// force `Content-Disposition: attachment; filename=...` on browser
// downloads). Overrides win over object metadata.
func applyResponseHeaderOverrides(w http.ResponseWriter, q map[string][]string) {
	overrides := map[string]string{
		"response-content-type":        "Content-Type",
		"response-content-disposition": "Content-Disposition",
		"response-cache-control":       "Cache-Control",
		"response-content-encoding":    "Content-Encoding",
		"response-content-language":    "Content-Language",
		"response-expires":             "Expires",
	}

	for queryKey, headerName := range overrides {
		if v := firstQueryValue(q, queryKey); v != "" {
			w.Header().Set(headerName, v)
		}
	}
}

func firstQueryValue(q map[string][]string, key string) string {
	if v, ok := q[key]; ok && len(v) > 0 {
		return v[0]
	}

	return ""
}

// setIfAbsent sets a response header only when the caller hasn't
// already set it. Used so that response-header overrides set by
// presigned-URL query parameters survive the default header writers.
func setIfAbsent(w http.ResponseWriter, name, value string) {
	if w.Header().Get(name) == "" {
		w.Header().Set(name, value)
	}
}

// writeNotModifiedResponse writes a 304 with the ETag and
// Last-Modified of the object so the cache can re-pin its entry per
// RFC 9111 §4.3.4. No body, no Content-Length.
func writeNotModifiedResponse(w http.ResponseWriter, obj *Object) {
	w.Header().Set("ETag", obj.ETag)
	w.Header().Set("Last-Modified", obj.LastModified.UTC().Format(timeFormatHTTP))
	w.WriteHeader(http.StatusNotModified)
}

// handleGetObjectError handles errors from GetObject/GetObjectVersion.
func handleGetObjectError(w http.ResponseWriter, r *http.Request, err error) {
	var bucketErr *BucketError
	if errors.As(err, &bucketErr) {
		writeS3Error(w, r, bucketErr.Code, bucketErr.Message, http.StatusNotFound)

		return
	}

	var objErr *ObjectError
	if errors.As(err, &objErr) {
		writeS3Error(w, r, objErr.Code, objErr.Message, http.StatusNotFound)

		return
	}

	writeS3Error(w, r, "InternalError", "Internal server error", http.StatusInternalServerError)
}

// writeObjectResponse writes the object response with headers and body.
// Pre-existing header values (e.g. set by applyResponseHeaderOverrides
// for presigned response-* overrides) are preserved.
func writeObjectResponse(w http.ResponseWriter, obj *Object) {
	setIfAbsent(w, "Content-Type", obj.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(obj.Size, 10))
	w.Header().Set("ETag", obj.ETag)
	w.Header().Set("Last-Modified", obj.LastModified.UTC().Format(timeFormatHTTP))
	w.Header().Set("Accept-Ranges", "bytes")

	if obj.VersionID != "" {
		w.Header().Set("x-amz-version-id", obj.VersionID)
	}

	for k, v := range obj.Metadata {
		if k != contentTypeHeader {
			w.Header().Set("x-amz-meta-"+k, v)
		}
	}

	w.WriteHeader(http.StatusOK)

	_, _ = w.Write(obj.Body)
}

// DeleteObject handles DELETE /{bucket}/{key...} - delete an object.
func (s *Service) DeleteObject(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	key := r.PathValue("key")

	if bucket == "" {
		writeS3Error(w, r, "InvalidBucketName", "The specified bucket is not valid.", http.StatusBadRequest)

		return
	}

	if key == "" {
		writeS3Error(w, r, "InvalidArgument", "Invalid key", http.StatusBadRequest)

		return
	}

	versionID := r.URL.Query().Get("versionId")

	var deleteMarker *Object

	var err error

	if versionID != "" {
		deleteMarker, err = s.storage.DeleteObjectVersion(r.Context(), bucket, key, versionID)
	} else {
		deleteMarker, err = s.storage.DeleteObject(r.Context(), bucket, key)
	}

	if err != nil {
		var bucketErr *BucketError
		if errors.As(err, &bucketErr) {
			writeS3Error(w, r, bucketErr.Code, bucketErr.Message, http.StatusNotFound)

			return
		}

		writeS3Error(w, r, "InternalError", "Internal server error", http.StatusInternalServerError)

		return
	}

	// Return version info in headers if applicable
	if deleteMarker != nil {
		if deleteMarker.VersionID != "" {
			w.Header().Set("x-amz-version-id", deleteMarker.VersionID)
		}

		if deleteMarker.IsDeleteMarker {
			w.Header().Set("x-amz-delete-marker", "true")
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// DeleteObjects handles POST /{bucket}?delete - delete multiple objects.
func (s *Service) DeleteObjects(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")

	if bucket == "" {
		writeS3Error(w, r, "InvalidBucketName", "The specified bucket is not valid.", http.StatusBadRequest)

		return
	}

	var req DeleteRequest
	if err := xml.NewDecoder(r.Body).Decode(&req); err != nil {
		writeS3Error(w, r, "MalformedXML", "The XML you provided was not well-formed", http.StatusBadRequest)

		return
	}

	result := DeleteResult{
		Xmlns: s3Namespace,
	}

	for _, obj := range req.Objects {
		s.deleteOneObject(r.Context(), bucket, obj, req.Quiet, &result)
	}

	writeXMLResponse(w, result)
}

// deleteOneObject processes a single object deletion for DeleteObjects.
func (s *Service) deleteOneObject(ctx context.Context, bucket string, obj DeleteObjectEntry, quiet bool, result *DeleteResult) {
	var deleteMarker *Object

	var err error

	if obj.VersionID != "" {
		deleteMarker, err = s.storage.DeleteObjectVersion(ctx, bucket, obj.Key, obj.VersionID)
	} else {
		deleteMarker, err = s.storage.DeleteObject(ctx, bucket, obj.Key)
	}

	if err != nil {
		var bucketErr *BucketError
		if errors.As(err, &bucketErr) {
			result.Errors = append(result.Errors, DeleteObjectError{
				Key: obj.Key, Code: bucketErr.Code, Message: bucketErr.Message, VersionID: obj.VersionID,
			})

			return
		}

		result.Errors = append(result.Errors, DeleteObjectError{
			Key: obj.Key, Code: "InternalError", Message: "Internal server error", VersionID: obj.VersionID,
		})

		return
	}

	if quiet {
		return
	}

	deleted := DeletedObject{Key: obj.Key}

	if deleteMarker != nil {
		if deleteMarker.VersionID != "" {
			deleted.VersionID = deleteMarker.VersionID
		}

		if deleteMarker.IsDeleteMarker {
			deleted.DeleteMarker = true
			deleted.DeleteMarkerVersionID = deleteMarker.VersionID
		}
	}

	result.Deleted = append(result.Deleted, deleted)
}

// HeadObject handles HEAD /{bucket}/{key...} - get object metadata.
func (s *Service) HeadObject(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	key := r.PathValue("key")

	if bucket == "" || key == "" {
		w.WriteHeader(http.StatusBadRequest)

		return
	}

	obj, err := s.storage.HeadObject(r.Context(), bucket, key)
	if err != nil {
		var bucketErr *BucketError
		if errors.As(err, &bucketErr) {
			w.WriteHeader(http.StatusNotFound)

			return
		}

		var objErr *ObjectError
		if errors.As(err, &objErr) {
			w.WriteHeader(http.StatusNotFound)

			return
		}

		w.WriteHeader(http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", obj.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(obj.Size, 10))
	w.Header().Set("ETag", obj.ETag)
	w.Header().Set("Last-Modified", obj.LastModified.UTC().Format(timeFormatHTTP))
	w.Header().Set("Accept-Ranges", "bytes")

	// Set metadata headers
	for k, v := range obj.Metadata {
		if k != contentTypeHeader {
			w.Header().Set("x-amz-meta-"+k, v)
		}
	}

	w.WriteHeader(http.StatusOK)
}

// PutBucketVersioning handles PUT /{bucket}?versioning - set bucket versioning.
func (s *Service) PutBucketVersioning(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	if bucket == "" {
		writeS3Error(w, r, "InvalidBucketName", "The specified bucket is not valid.", http.StatusBadRequest)

		return
	}

	var config VersioningConfiguration
	if err := xml.NewDecoder(r.Body).Decode(&config); err != nil {
		writeS3Error(w, r, "MalformedXML", "The XML you provided was not well-formed", http.StatusBadRequest)

		return
	}

	err := s.storage.PutBucketVersioning(r.Context(), bucket, config.Status)
	if err != nil {
		var bucketErr *BucketError
		if errors.As(err, &bucketErr) {
			writeS3Error(w, r, bucketErr.Code, bucketErr.Message, http.StatusNotFound)

			return
		}

		writeS3Error(w, r, "InternalError", "Internal server error", http.StatusInternalServerError)

		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetBucketVersioning handles GET /{bucket}?versioning - get bucket versioning.
func (s *Service) GetBucketVersioning(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	if bucket == "" {
		writeS3Error(w, r, "InvalidBucketName", "The specified bucket is not valid.", http.StatusBadRequest)

		return
	}

	status, err := s.storage.GetBucketVersioning(r.Context(), bucket)
	if err != nil {
		var bucketErr *BucketError
		if errors.As(err, &bucketErr) {
			writeS3Error(w, r, bucketErr.Code, bucketErr.Message, http.StatusNotFound)

			return
		}

		writeS3Error(w, r, "InternalError", "Internal server error", http.StatusInternalServerError)

		return
	}

	result := VersioningConfiguration{
		Xmlns:  s3Namespace,
		Status: status,
	}

	writeXMLResponse(w, result)
}

// ListObjectVersions handles GET /{bucket}?versions - list object versions.
func (s *Service) ListObjectVersions(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	if bucket == "" {
		writeS3Error(w, r, "InvalidBucketName", "The specified bucket is not valid.", http.StatusBadRequest)

		return
	}

	prefix := r.URL.Query().Get("prefix")
	delimiter := r.URL.Query().Get("delimiter")
	maxKeys := parseMaxKeys(r.URL.Query().Get("max-keys"))

	objects, commonPrefixes, err := s.storage.ListObjectVersions(r.Context(), bucket, prefix, delimiter, maxKeys)
	if err != nil {
		handleListVersionsError(w, r, err)

		return
	}

	versions, deleteMarkers := separateVersionsAndDeleteMarkers(objects)
	prefixes := toCommonPrefixes(commonPrefixes)

	result := ListVersionsResult{
		Xmlns:          s3Namespace,
		Name:           bucket,
		Prefix:         prefix,
		MaxKeys:        maxKeys,
		IsTruncated:    false,
		Versions:       versions,
		DeleteMarkers:  deleteMarkers,
		CommonPrefixes: prefixes,
	}

	writeXMLResponse(w, result)
}

// parseMaxKeys parses max-keys query parameter with default of 1000.
func parseMaxKeys(maxKeysStr string) int {
	if maxKeysStr == "" {
		return 1000
	}

	if mk, err := strconv.Atoi(maxKeysStr); err == nil && mk > 0 {
		return mk
	}

	return 1000
}

// handleListVersionsError handles errors from ListObjectVersions.
func handleListVersionsError(w http.ResponseWriter, r *http.Request, err error) {
	var bucketErr *BucketError
	if errors.As(err, &bucketErr) {
		writeS3Error(w, r, bucketErr.Code, bucketErr.Message, http.StatusNotFound)

		return
	}

	writeS3Error(w, r, "InternalError", "Internal server error", http.StatusInternalServerError)
}

// separateVersionsAndDeleteMarkers separates objects into versions and delete markers.
func separateVersionsAndDeleteMarkers(objects []Object) ([]ObjectVersionInfo, []DeleteMarkerInfo) {
	versions := make([]ObjectVersionInfo, 0, len(objects))
	deleteMarkers := make([]DeleteMarkerInfo, 0)

	for i := range objects {
		obj := &objects[i]
		isLatest := i == 0 || objects[i-1].Key != obj.Key

		if obj.IsDeleteMarker {
			deleteMarkers = append(deleteMarkers, toDeleteMarkerInfo(obj, isLatest))
		} else {
			versions = append(versions, toObjectVersionInfo(obj, isLatest))
		}
	}

	return versions, deleteMarkers
}

// toObjectVersionInfo converts an Object to ObjectVersionInfo.
func toObjectVersionInfo(obj *Object, isLatest bool) ObjectVersionInfo {
	return ObjectVersionInfo{
		Key:          obj.Key,
		VersionID:    obj.VersionID,
		IsLatest:     isLatest,
		LastModified: obj.LastModified.Format(timeFormatISO),
		ETag:         obj.ETag,
		Size:         obj.Size,
		StorageClass: "STANDARD",
		Owner:        Owner{ID: "owner-id"},
	}
}

// toDeleteMarkerInfo converts an Object to DeleteMarkerInfo.
func toDeleteMarkerInfo(obj *Object, isLatest bool) DeleteMarkerInfo {
	return DeleteMarkerInfo{
		Key:          obj.Key,
		VersionID:    obj.VersionID,
		IsLatest:     isLatest,
		LastModified: obj.LastModified.Format(timeFormatISO),
		Owner:        Owner{ID: "owner-id"},
	}
}

// toCommonPrefixes converts string slice to CommonPrefix slice.
func toCommonPrefixes(prefixes []string) []CommonPrefix {
	result := make([]CommonPrefix, len(prefixes))
	for i, p := range prefixes {
		result[i] = CommonPrefix{Prefix: p}
	}

	return result
}

// handleBucketPut routes PUT /{bucket} requests based on query parameters.
func (s *Service) handleBucketPut(w http.ResponseWriter, r *http.Request) {
	if _, ok := r.URL.Query()["versioning"]; ok {
		s.PutBucketVersioning(w, r)

		return
	}

	if _, ok := r.URL.Query()["notification"]; ok {
		s.PutBucketNotificationConfiguration(w, r)

		return
	}

	if _, ok := r.URL.Query()["cors"]; ok {
		s.PutBucketCors(w, r)

		return
	}

	if _, ok := r.URL.Query()["publicAccessBlock"]; ok {
		s.PutPublicAccessBlock(w, r)

		return
	}

	if _, ok := r.URL.Query()["encryption"]; ok {
		s.PutBucketEncryption(w, r)

		return
	}

	if _, ok := r.URL.Query()["policy"]; ok {
		s.PutBucketPolicy(w, r)

		return
	}

	if _, ok := r.URL.Query()["logging"]; ok {
		s.PutBucketLogging(w, r)

		return
	}

	s.CreateBucket(w, r)
}

// handleBucketDelete dispatches DELETE /{bucket} requests based on query parameters.
func (s *Service) handleBucketDelete(w http.ResponseWriter, r *http.Request) {
	if _, ok := r.URL.Query()["publicAccessBlock"]; ok {
		s.DeletePublicAccessBlock(w, r)

		return
	}

	if _, ok := r.URL.Query()["encryption"]; ok {
		s.DeleteBucketEncryption(w, r)

		return
	}

	if _, ok := r.URL.Query()["policy"]; ok {
		s.DeleteBucketPolicy(w, r)

		return
	}

	s.DeleteBucket(w, r)
}

// PutPublicAccessBlock handles PUT /{bucket}?publicAccessBlock.
func (s *Service) PutPublicAccessBlock(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeS3Error(w, r, "InvalidRequest", "Failed to read request body", http.StatusBadRequest)

		return
	}

	var cfg PublicAccessBlockConfiguration
	if err := xml.Unmarshal(body, &cfg); err != nil {
		writeS3Error(w, r, "MalformedXML", "The request body is malformed XML", http.StatusBadRequest)

		return
	}

	if err := s.storage.PutPublicAccessBlock(r.Context(), bucket, PublicAccessBlockConfig{
		BlockPublicAcls:       cfg.BlockPublicAcls,
		IgnorePublicAcls:      cfg.IgnorePublicAcls,
		BlockPublicPolicy:     cfg.BlockPublicPolicy,
		RestrictPublicBuckets: cfg.RestrictPublicBuckets,
	}); err != nil {
		writeBucketErrorOrInternal(w, r, err)

		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetPublicAccessBlock handles GET /{bucket}?publicAccessBlock.
func (s *Service) GetPublicAccessBlock(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")

	cfg, err := s.storage.GetPublicAccessBlock(r.Context(), bucket)
	if err != nil {
		writeBucketErrorOrInternal(w, r, err)

		return
	}

	writeXMLResponse(w, PublicAccessBlockConfiguration{
		Xmlns:                 s3Namespace,
		BlockPublicAcls:       cfg.BlockPublicAcls,
		IgnorePublicAcls:      cfg.IgnorePublicAcls,
		BlockPublicPolicy:     cfg.BlockPublicPolicy,
		RestrictPublicBuckets: cfg.RestrictPublicBuckets,
	})
}

// DeletePublicAccessBlock handles DELETE /{bucket}?publicAccessBlock.
func (s *Service) DeletePublicAccessBlock(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")

	if err := s.storage.DeletePublicAccessBlock(r.Context(), bucket); err != nil {
		writeBucketErrorOrInternal(w, r, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// PutBucketEncryption handles PUT /{bucket}?encryption.
func (s *Service) PutBucketEncryption(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeS3Error(w, r, "InvalidRequest", "Failed to read request body", http.StatusBadRequest)

		return
	}

	var cfg ServerSideEncryptionConfiguration
	if err := xml.Unmarshal(body, &cfg); err != nil {
		writeS3Error(w, r, "MalformedXML", "The request body is malformed XML", http.StatusBadRequest)

		return
	}

	rules := make([]ServerSideEncryptionRule, 0, len(cfg.Rules))

	for _, rule := range cfg.Rules {
		stored := ServerSideEncryptionRule{BucketKeyEnabled: rule.BucketKeyEnabled}
		if rule.ApplyServerSideEncryptionByDefault != nil {
			stored.SSEAlgorithm = rule.ApplyServerSideEncryptionByDefault.SSEAlgorithm
			stored.KMSMasterKeyID = rule.ApplyServerSideEncryptionByDefault.KMSMasterKeyID
		}

		rules = append(rules, stored)
	}

	if err := s.storage.PutBucketEncryption(r.Context(), bucket, ServerSideEncryptionConfig{Rules: rules}); err != nil {
		writeBucketErrorOrInternal(w, r, err)

		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetBucketEncryption handles GET /{bucket}?encryption.
func (s *Service) GetBucketEncryption(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")

	cfg, err := s.storage.GetBucketEncryption(r.Context(), bucket)
	if err != nil {
		writeBucketErrorOrInternal(w, r, err)

		return
	}

	xmlRules := make([]ServerSideEncryptionRuleX, 0, len(cfg.Rules))

	for _, rule := range cfg.Rules {
		xmlRule := ServerSideEncryptionRuleX{BucketKeyEnabled: rule.BucketKeyEnabled}
		if rule.SSEAlgorithm != "" {
			xmlRule.ApplyServerSideEncryptionByDefault = &ApplyServerSideEncryptionByDefault{
				SSEAlgorithm:   rule.SSEAlgorithm,
				KMSMasterKeyID: rule.KMSMasterKeyID,
			}
		}

		xmlRules = append(xmlRules, xmlRule)
	}

	writeXMLResponse(w, ServerSideEncryptionConfiguration{
		Xmlns: s3Namespace,
		Rules: xmlRules,
	})
}

// PutBucketLogging handles PUT /{bucket}?logging. Body is the AWS
// BucketLoggingStatus XML; an empty / missing LoggingEnabled element
// disables logging on the bucket. terraform aws_s3_bucket_logging is
// the primary caller.
func (s *Service) PutBucketLogging(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeS3Error(w, r, "InvalidRequest", "Failed to read request body", http.StatusBadRequest)

		return
	}

	var status BucketLoggingStatus
	if len(body) > 0 {
		if err := xml.Unmarshal(body, &status); err != nil {
			writeS3Error(w, r, "MalformedXML", "Failed to parse BucketLoggingStatus", http.StatusBadRequest)

			return
		}
	}

	cfg := BucketLoggingConfig{}
	if status.LoggingEnabled != nil {
		cfg.TargetBucket = status.LoggingEnabled.TargetBucket
		cfg.TargetPrefix = status.LoggingEnabled.TargetPrefix
	}

	if err := s.storage.PutBucketLogging(r.Context(), bucket, cfg); err != nil {
		writeBucketErrorOrInternal(w, r, err)

		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetBucketLogging handles GET /{bucket}?logging. Returns the wire
// shape with LoggingEnabled present when configured, or an empty
// status element when not — matching real AWS behaviour. terraform
// reads this back to detect drift.
func (s *Service) GetBucketLogging(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")

	cfg, err := s.storage.GetBucketLogging(r.Context(), bucket)
	if err != nil {
		writeBucketErrorOrInternal(w, r, err)

		return
	}

	resp := BucketLoggingStatus{Xmlns: s3Namespace}
	if cfg != nil {
		resp.LoggingEnabled = &LoggingEnabledStatus{
			TargetBucket: cfg.TargetBucket,
			TargetPrefix: cfg.TargetPrefix,
		}
	}

	writeXMLResponse(w, resp)
}

// DeleteBucketEncryption handles DELETE /{bucket}?encryption.
func (s *Service) DeleteBucketEncryption(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")

	if err := s.storage.DeleteBucketEncryption(r.Context(), bucket); err != nil {
		writeBucketErrorOrInternal(w, r, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// PutBucketPolicy handles PUT /{bucket}?policy. The body is the
// raw JSON policy document — AWS treats it as opaque for storage,
// so we don't validate structure here. terraform aws_s3_bucket_policy
// is the primary caller.
func (s *Service) PutBucketPolicy(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeS3Error(w, r, "InvalidArgument", "Failed to read request body", http.StatusBadRequest)

		return
	}

	if err := s.storage.PutBucketPolicy(r.Context(), bucket, string(body)); err != nil {
		writeBucketErrorOrInternal(w, r, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetBucketPolicy handles GET /{bucket}?policy. Returns the raw
// document as application/json (mirroring real S3) or NoSuchBucketPolicy
// when the bucket exists without a policy.
func (s *Service) GetBucketPolicy(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")

	doc, err := s.storage.GetBucketPolicy(r.Context(), bucket)
	if err != nil {
		writeBucketErrorOrInternal(w, r, err)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(doc))
}

// DeleteBucketPolicy handles DELETE /{bucket}?policy. AWS returns 204
// even when no policy was set, so a missing policy is not an error.
func (s *Service) DeleteBucketPolicy(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")

	if err := s.storage.DeleteBucketPolicy(r.Context(), bucket); err != nil {
		writeBucketErrorOrInternal(w, r, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// writeBucketErrorOrInternal maps a BucketError to its HTTP status code,
// falling back to 500 for non-bucket errors. Used by sub-resource handlers.
func writeBucketErrorOrInternal(w http.ResponseWriter, r *http.Request, err error) {
	var bucketErr *BucketError
	if errors.As(err, &bucketErr) {
		status := http.StatusBadRequest

		switch bucketErr.Code {
		case "NoSuchBucket", "NoSuchPublicAccessBlockConfiguration", "ServerSideEncryptionConfigurationNotFoundError", "NoSuchBucketPolicy":
			status = http.StatusNotFound
		}

		writeS3Error(w, r, bucketErr.Code, bucketErr.Message, status)

		return
	}

	writeS3Error(w, r, "InternalError", "Internal server error", http.StatusInternalServerError)
}

// writeXMLResponse writes an XML response with HTTP 200 OK status.
func writeXMLResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)

	_, _ = io.WriteString(w, xmlHeader)
	_ = xml.NewEncoder(w).Encode(v)
}

// writeS3Error writes an S3 error response.
func writeS3Error(w http.ResponseWriter, _ *http.Request, code, message string, status int) {
	errResp := ErrorResponse{
		Code:      code,
		Message:   message,
		RequestID: uuid.New().String(),
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)

	_, _ = io.WriteString(w, xmlHeader)
	_ = xml.NewEncoder(w).Encode(errResp)
}

// isPresignedRequest checks if the request is a presigned URL request.
func isPresignedRequest(r *http.Request) bool {
	return r.URL.Query().Get("X-Amz-Signature") != ""
}

// checkPresignedURL validates presigned URL if present and writes error response if invalid.
// Returns true if the request should continue processing, false if an error was written.
func checkPresignedURL(w http.ResponseWriter, r *http.Request) bool {
	if !isPresignedRequest(r) {
		return true
	}

	if err := validatePresignedURL(r); err != nil {
		var presignErr *PresignedURLError
		if errors.As(err, &presignErr) {
			writeS3Error(w, r, presignErr.Code, presignErr.Message, http.StatusForbidden)

			return false
		}

		writeS3Error(w, r, "InternalError", "Internal server error", http.StatusInternalServerError)

		return false
	}

	return true
}

// validatePresignedURL validates the presigned URL expiration.
// Returns nil if the URL is valid, or an error if expired.
func validatePresignedURL(r *http.Request) error {
	// Get the date when the URL was signed
	amzDate := r.URL.Query().Get("X-Amz-Date")
	if amzDate == "" {
		return &PresignedURLError{Code: "AuthorizationQueryParametersError", Message: "X-Amz-Date must be in the ISO8601 Long Format"}
	}

	// Get the expiration in seconds
	expiresStr := r.URL.Query().Get("X-Amz-Expires")
	if expiresStr == "" {
		return &PresignedURLError{Code: "AuthorizationQueryParametersError", Message: "X-Amz-Expires must be provided"}
	}

	expires, err := strconv.ParseInt(expiresStr, 10, 64)
	if err != nil {
		return &PresignedURLError{Code: "AuthorizationQueryParametersError", Message: "X-Amz-Expires must be a number"}
	}

	// AWS allows max 7 days (604800 seconds) for presigned URLs
	const maxExpires = 604800
	if expires > maxExpires {
		return &PresignedURLError{Code: "AuthorizationQueryParametersError", Message: "X-Amz-Expires must be less than 604800 seconds"}
	}

	// Parse the signing date (format: 20060102T150405Z)
	signTime, err := time.Parse("20060102T150405Z", amzDate)
	if err != nil {
		return &PresignedURLError{Code: "AuthorizationQueryParametersError", Message: "Invalid X-Amz-Date format"}
	}

	// Check if the URL has expired
	expirationTime := signTime.Add(time.Duration(expires) * time.Second)
	if time.Now().After(expirationTime) {
		return &PresignedURLError{Code: "AccessDenied", Message: "Request has expired"}
	}

	return nil
}

// PresignedURLError represents a presigned URL validation error.
type PresignedURLError struct {
	Code    string
	Message string
}

func (e *PresignedURLError) Error() string {
	return e.Code + ": " + e.Message
}

// Multipart Upload Handlers

// CreateMultipartUpload handles POST /{bucket}/{key}?uploads - initiate a multipart upload.
func (s *Service) CreateMultipartUpload(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	key := r.PathValue("key")

	if bucket == "" {
		writeS3Error(w, r, "InvalidBucketName", "The specified bucket is not valid.", http.StatusBadRequest)

		return
	}

	if key == "" {
		writeS3Error(w, r, "InvalidArgument", "Invalid key", http.StatusBadRequest)

		return
	}

	upload, err := s.storage.CreateMultipartUpload(r.Context(), bucket, key)
	if err != nil {
		var bucketErr *BucketError
		if errors.As(err, &bucketErr) {
			writeS3Error(w, r, bucketErr.Code, bucketErr.Message, http.StatusNotFound)

			return
		}

		writeS3Error(w, r, "InternalError", "Internal server error", http.StatusInternalServerError)

		return
	}

	result := InitiateMultipartUploadResult{
		Xmlns:    s3Namespace,
		Bucket:   bucket,
		Key:      key,
		UploadID: upload.UploadID,
	}

	writeXMLResponse(w, result)
}

// UploadPart handles PUT /{bucket}/{key}?partNumber={partNumber}&uploadId={uploadId} - upload a part.
func (s *Service) UploadPart(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	key := r.PathValue("key")

	if bucket == "" {
		writeS3Error(w, r, "InvalidBucketName", "The specified bucket is not valid.", http.StatusBadRequest)

		return
	}

	if key == "" {
		writeS3Error(w, r, "InvalidArgument", "Invalid key", http.StatusBadRequest)

		return
	}

	uploadID := r.URL.Query().Get("uploadId")
	if uploadID == "" {
		writeS3Error(w, r, "InvalidArgument", "uploadId is required", http.StatusBadRequest)

		return
	}

	partNumberStr := r.URL.Query().Get("partNumber")
	partNumber, err := strconv.Atoi(partNumberStr)

	if err != nil || partNumber < 1 || partNumber > 10000 {
		writeS3Error(w, r, "InvalidArgument", "Invalid partNumber", http.StatusBadRequest)

		return
	}

	part, err := s.storage.UploadPart(r.Context(), bucket, key, uploadID, partNumber, r.Body)
	if err != nil {
		handleMultipartError(w, r, err)

		return
	}

	w.Header().Set("ETag", part.ETag)
	w.WriteHeader(http.StatusOK)
}

// UploadPartCopy handles
//
//	PUT /{bucket}/{key}?partNumber=N&uploadId=...
//	X-Amz-Copy-Source: /<srcBucket>/<srcKey>
//	X-Amz-Copy-Source-Range: bytes=START-END   (optional)
//
// — copies bytes from an existing object into a part of an in-progress
// multipart upload, without the client having to re-upload the data.
//
// Cross-bucket copy is supported. Source object must already exist.
// If `X-Amz-Copy-Source-Range` is absent, the whole source is copied.
func (s *Service) UploadPartCopy(w http.ResponseWriter, r *http.Request) {
	dstBucket := r.PathValue("bucket")
	dstKey := r.PathValue("key")

	uploadID := r.URL.Query().Get("uploadId")
	if uploadID == "" {
		writeS3Error(w, r, "InvalidArgument", "uploadId is required", http.StatusBadRequest)

		return
	}

	partNumber, err := strconv.Atoi(r.URL.Query().Get("partNumber"))
	if err != nil || partNumber < 1 || partNumber > 10000 {
		writeS3Error(w, r, "InvalidArgument", "Invalid partNumber", http.StatusBadRequest)

		return
	}

	srcBucket, srcKey := parseCopySource(r.Header.Get("X-Amz-Copy-Source"))
	if srcBucket == "" || srcKey == "" {
		writeS3Error(w, r, "InvalidArgument", "Invalid copy source", http.StatusBadRequest)

		return
	}

	copyRange, err := parseCopySourceRange(r.Header.Get("X-Amz-Copy-Source-Range"))
	if err != nil {
		writeS3Error(w, r, "InvalidArgument", err.Error(), http.StatusBadRequest)

		return
	}

	part, err := s.storage.UploadPartCopy(r.Context(), dstBucket, dstKey, uploadID, partNumber, srcBucket, srcKey, copyRange)
	if err != nil {
		handleMultipartError(w, r, err)

		return
	}

	result := CopyPartResult{
		Xmlns:        s3Namespace,
		LastModified: part.LastModified.UTC().Format(timeFormatISO),
		ETag:         part.ETag,
	}
	writeXMLResponse(w, result)
}

// parseCopySourceRange parses an `X-Amz-Copy-Source-Range: bytes=START-END`
// header. Returns (nil, nil) when absent. Suffix / open-ended forms are
// not allowed by S3 for UploadPartCopy — only closed ranges.
func parseCopySourceRange(header string) (*CopyRange, error) {
	if header == "" {
		return nil, nil //nolint:nilnil // absent header is the "copy whole source" sentinel
	}

	spec, ok := strings.CutPrefix(header, "bytes=")
	if !ok {
		return nil, fmt.Errorf("X-Amz-Copy-Source-Range must use bytes unit")
	}

	dash := strings.IndexByte(spec, '-')
	if dash <= 0 || dash == len(spec)-1 {
		return nil, fmt.Errorf("X-Amz-Copy-Source-Range requires both START and END (closed range)")
	}

	start, err1 := strconv.ParseInt(strings.TrimSpace(spec[:dash]), 10, 64)
	end, err2 := strconv.ParseInt(strings.TrimSpace(spec[dash+1:]), 10, 64)

	if err1 != nil || err2 != nil || start < 0 || end < start {
		return nil, fmt.Errorf("X-Amz-Copy-Source-Range has invalid byte range")
	}

	return &CopyRange{Start: start, End: end}, nil
}

// CompleteMultipartUpload handles POST /{bucket}/{key}?uploadId={uploadId} - complete a multipart upload.
func (s *Service) CompleteMultipartUpload(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	key := r.PathValue("key")

	if bucket == "" {
		writeS3Error(w, r, "InvalidBucketName", "The specified bucket is not valid.", http.StatusBadRequest)

		return
	}

	if key == "" {
		writeS3Error(w, r, "InvalidArgument", "Invalid key", http.StatusBadRequest)

		return
	}

	uploadID := r.URL.Query().Get("uploadId")
	if uploadID == "" {
		writeS3Error(w, r, "InvalidArgument", "uploadId is required", http.StatusBadRequest)

		return
	}

	// Parse the XML request body
	var req CompleteMultipartUploadRequest
	if err := xml.NewDecoder(r.Body).Decode(&req); err != nil {
		writeS3Error(w, r, "MalformedXML", "The XML you provided was not well-formed", http.StatusBadRequest)

		return
	}

	obj, err := s.storage.CompleteMultipartUpload(r.Context(), bucket, key, uploadID, req.Parts)
	if err != nil {
		handleMultipartError(w, r, err)

		return
	}

	result := CompleteMultipartUploadResult{
		Xmlns:    s3Namespace,
		Location: "/" + bucket + "/" + key,
		Bucket:   bucket,
		Key:      key,
		ETag:     obj.ETag,
	}

	writeXMLResponse(w, result)
}

// AbortMultipartUpload handles DELETE /{bucket}/{key}?uploadId={uploadId} - abort a multipart upload.
func (s *Service) AbortMultipartUpload(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	key := r.PathValue("key")

	if bucket == "" {
		writeS3Error(w, r, "InvalidBucketName", "The specified bucket is not valid.", http.StatusBadRequest)

		return
	}

	if key == "" {
		writeS3Error(w, r, "InvalidArgument", "Invalid key", http.StatusBadRequest)

		return
	}

	uploadID := r.URL.Query().Get("uploadId")
	if uploadID == "" {
		writeS3Error(w, r, "InvalidArgument", "uploadId is required", http.StatusBadRequest)

		return
	}

	err := s.storage.AbortMultipartUpload(r.Context(), bucket, key, uploadID)
	if err != nil {
		handleMultipartError(w, r, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListMultipartUploads handles GET /{bucket}?uploads - list in-progress multipart uploads.
func (s *Service) ListMultipartUploads(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	if bucket == "" {
		writeS3Error(w, r, "InvalidBucketName", "The specified bucket is not valid.", http.StatusBadRequest)

		return
	}

	prefix := r.URL.Query().Get("prefix")
	maxUploads := 1000

	if maxUploadsStr := r.URL.Query().Get("max-uploads"); maxUploadsStr != "" {
		if mu, err := strconv.Atoi(maxUploadsStr); err == nil && mu > 0 {
			maxUploads = mu
		}
	}

	uploads, err := s.storage.ListMultipartUploads(r.Context(), bucket, prefix, maxUploads)
	if err != nil {
		var bucketErr *BucketError
		if errors.As(err, &bucketErr) {
			writeS3Error(w, r, bucketErr.Code, bucketErr.Message, http.StatusNotFound)

			return
		}

		writeS3Error(w, r, "InternalError", "Internal server error", http.StatusInternalServerError)

		return
	}

	uploadInfos := make([]UploadInfo, len(uploads))
	for i, u := range uploads {
		uploadInfos[i] = UploadInfo{
			Key:       u.Key,
			UploadID:  u.UploadID,
			Initiated: u.Initiated.Format(timeFormatISO),
		}
	}

	result := ListMultipartUploadsResult{
		Xmlns:       s3Namespace,
		Bucket:      bucket,
		MaxUploads:  maxUploads,
		IsTruncated: false,
		Uploads:     uploadInfos,
	}

	writeXMLResponse(w, result)
}

// ListParts handles GET /{bucket}/{key}?uploadId={uploadId} - list parts of a multipart upload.
func (s *Service) ListParts(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	key := r.PathValue("key")

	if bucket == "" {
		writeS3Error(w, r, "InvalidBucketName", "The specified bucket is not valid.", http.StatusBadRequest)

		return
	}

	if key == "" {
		writeS3Error(w, r, "InvalidArgument", "Invalid key", http.StatusBadRequest)

		return
	}

	uploadID := r.URL.Query().Get("uploadId")
	if uploadID == "" {
		writeS3Error(w, r, "InvalidArgument", "uploadId is required", http.StatusBadRequest)

		return
	}

	maxParts := 1000

	if maxPartsStr := r.URL.Query().Get("max-parts"); maxPartsStr != "" {
		if mp, err := strconv.Atoi(maxPartsStr); err == nil && mp > 0 {
			maxParts = mp
		}
	}

	parts, err := s.storage.ListParts(r.Context(), bucket, key, uploadID, maxParts)
	if err != nil {
		handleMultipartError(w, r, err)

		return
	}

	partInfos := make([]PartInfo, len(parts))
	for i, p := range parts {
		partInfos[i] = PartInfo{
			PartNumber:   p.PartNumber,
			LastModified: p.LastModified.Format(timeFormatISO),
			ETag:         p.ETag,
			Size:         p.Size,
		}
	}

	result := ListPartsResult{
		Xmlns:       s3Namespace,
		Bucket:      bucket,
		Key:         key,
		UploadID:    uploadID,
		MaxParts:    maxParts,
		IsTruncated: false,
		Parts:       partInfos,
	}

	writeXMLResponse(w, result)
}

// PutBucketNotificationConfiguration handles PUT /{bucket}?notification.
func (s *Service) PutBucketNotificationConfiguration(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")

	var config NotificationConfiguration
	if err := xml.NewDecoder(r.Body).Decode(&config); err != nil {
		// Accept even if body is empty or malformed (AWS is lenient).
		w.WriteHeader(http.StatusOK)

		return
	}

	enabled := config.EventBridgeConfig != nil
	s.storage.SetEventBridgeNotification(r.Context(), bucket, enabled)

	w.WriteHeader(http.StatusOK)
}

// PutBucketCors handles PUT /{bucket}?cors.
func (s *Service) PutBucketCors(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")

	var config CORSConfiguration
	if err := xml.NewDecoder(r.Body).Decode(&config); err != nil {
		writeS3Error(w, r, "MalformedXML", "The XML you provided was not well-formed", http.StatusBadRequest)

		return
	}

	s.storage.SetCORSConfiguration(r.Context(), bucket, config.CORSRules)

	w.WriteHeader(http.StatusOK)
}

// handleMultipartError handles errors from multipart upload operations.
func handleMultipartError(w http.ResponseWriter, r *http.Request, err error) {
	var bucketErr *BucketError
	if errors.As(err, &bucketErr) {
		writeS3Error(w, r, bucketErr.Code, bucketErr.Message, http.StatusNotFound)

		return
	}

	var objectErr *ObjectError
	if errors.As(err, &objectErr) {
		writeS3Error(w, r, objectErr.Code, objectErr.Message, http.StatusNotFound)

		return
	}

	var multipartErr *MultipartError
	if errors.As(err, &multipartErr) {
		status := http.StatusNotFound
		if multipartErr.Code == "InvalidPart" || multipartErr.Code == "InvalidArgument" {
			status = http.StatusBadRequest
		}

		writeS3Error(w, r, multipartErr.Code, multipartErr.Message, status)

		return
	}

	writeS3Error(w, r, "InternalError", "Internal server error", http.StatusInternalServerError)
}

// PutObjectTagging handles PUT /{bucket}/{key}?tagging.
func (s *Service) PutObjectTagging(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	key := r.PathValue("key")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeS3Error(w, r, "InternalError", "Failed to read request body", http.StatusInternalServerError)

		return
	}

	var tagging Tagging
	if err := xml.Unmarshal(body, &tagging); err != nil {
		writeS3Error(w, r, "MalformedXML", "Invalid XML in request body", http.StatusBadRequest)

		return
	}

	tags := make(map[string]string, len(tagging.TagSet.Tags))
	for _, tag := range tagging.TagSet.Tags {
		tags[tag.Key] = tag.Value
	}

	if err := s.storage.PutObjectTagging(r.Context(), bucket, key, tags); err != nil {
		var bErr *BucketError
		if errors.As(err, &bErr) {
			writeS3Error(w, r, bErr.Code, bErr.Message, http.StatusNotFound)

			return
		}

		writeS3Error(w, r, "InternalError", "Internal server error", http.StatusInternalServerError)

		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetObjectTagging handles GET /{bucket}/{key}?tagging.
func (s *Service) GetObjectTagging(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	key := r.PathValue("key")

	tags, err := s.storage.GetObjectTagging(r.Context(), bucket, key)
	if err != nil {
		var bErr *BucketError
		if errors.As(err, &bErr) {
			writeS3Error(w, r, bErr.Code, bErr.Message, http.StatusNotFound)

			return
		}

		writeS3Error(w, r, "InternalError", "Internal server error", http.StatusInternalServerError)

		return
	}

	tagging := Tagging{TagSet: TagSet{Tags: make([]Tag, 0, len(tags))}}

	keys := make([]string, 0, len(tags))
	for k := range tags {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, k := range keys {
		tagging.TagSet.Tags = append(tagging.TagSet.Tags, Tag{Key: k, Value: tags[k]})
	}

	w.Header().Set("Content-Type", "application/xml")

	resp, _ := xml.Marshal(tagging)
	_, _ = w.Write(resp)
}
