package s3

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
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

	if _, ok := r.URL.Query()["versions"]; ok {
		s.ListObjectVersions(w, r)

		return
	}

	if _, ok := r.URL.Query()["uploads"]; ok {
		s.ListMultipartUploads(w, r)

		return
	}

	s.ListObjects(w, r)
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

	if r.URL.Query().Has("tagging") {
		s.PutObjectTagging(w, r)

		return
	}

	if r.URL.Query().Get("uploadId") != "" && r.URL.Query().Get("partNumber") != "" {
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

	writeObjectResponse(w, obj)
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
func writeObjectResponse(w http.ResponseWriter, obj *Object) {
	w.Header().Set("Content-Type", obj.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(obj.Size, 10))
	w.Header().Set("ETag", obj.ETag)
	w.Header().Set("Last-Modified", obj.LastModified.UTC().Format(timeFormatHTTP))

	if obj.VersionID != "" {
		w.Header().Set("x-amz-version-id", obj.VersionID)
	}

	for k, v := range obj.Metadata {
		if k != "Content-Type" {
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

	// Set metadata headers
	for k, v := range obj.Metadata {
		if k != "Content-Type" {
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

	s.CreateBucket(w, r)
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

	var multipartErr *MultipartError
	if errors.As(err, &multipartErr) {
		status := http.StatusNotFound
		if multipartErr.Code == "InvalidPart" {
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
