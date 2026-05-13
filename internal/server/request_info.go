package server

import (
	"bytes"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/sivchari/kumo/internal/awsapi"
)

type requestInfo struct {
	awsapi.RequestInfo
	isControl bool
}

func (r *Router) requestInfo(routeService, method, pattern string, req *http.Request) requestInfo {
	info := requestInfo{
		RequestInfo: awsapi.RequestInfo{
			Service: r.catalog.MustNormalize(routeService),
			Method:  method,
			Path:    req.URL.Path,
			Pattern: pattern,
		},
		isControl: isControlPath(req.URL.Path),
	}

	if info.isControl {
		info.Service = "kumo"
		info.Protocol = "control"

		return info
	}

	info.Protocol = "rest"

	if target := req.Header.Get("X-Amz-Target"); target != "" {
		r.applyJSONInfo(&info, target)

		return info
	}

	if isQueryProtocolRequest(req) {
		r.applyQueryInfo(&info, req)

		return info
	}

	if serviceName, operation, ok := parseCBORPath(req.URL.Path); ok {
		r.applyCBORInfo(&info, serviceName, operation)

		return info
	}

	info.Action = inferRESTAction(info.Service, method, pattern, req)
	info.Resource = inferResource(info.Service, req)

	return info
}

func (r *Router) applyJSONInfo(info *requestInfo, target string) {
	parts := strings.SplitN(target, ".", 2)
	if len(parts) != 2 {
		return
	}

	info.Protocol = "json"
	info.Action = parts[1]

	service, ok := r.jsonPrefixes[parts[0]]
	if ok {
		info.Service = service

		return
	}

	info.Service = r.catalog.MustNormalize(parts[0])
}

func (r *Router) applyQueryInfo(info *requestInfo, req *http.Request) {
	info.Protocol = "query"
	info.Action = peekFormValue(req, "Action")

	svcID := parseServiceFromUserAgent(req.Header.Get("User-Agent"))
	if svcID != "" {
		info.Service = r.catalog.MustNormalize(svcID)
	}
}

func (r *Router) applyCBORInfo(info *requestInfo, serviceName, operation string) {
	info.Protocol = "cbor"
	info.Action = operation

	service, ok := r.cborNames[serviceName]
	if ok {
		info.Service = service

		return
	}

	info.Service = r.catalog.MustNormalize(serviceName)
}

func isControlPath(path string) bool {
	return path == "/health" || path == "/kumo" || strings.HasPrefix(path, "/kumo/")
}

func isQueryProtocolRequest(req *http.Request) bool {
	mediaType, _, _ := mime.ParseMediaType(req.Header.Get("Content-Type"))

	return mediaType == "application/x-www-form-urlencoded"
}

func peekFormValue(req *http.Request, key string) string {
	if value := req.URL.Query().Get(key); value != "" {
		return value
	}

	if req.Body == nil {
		return ""
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return ""
	}

	req.Body = io.NopCloser(bytes.NewReader(body))

	values, err := url.ParseQuery(string(body))
	if err != nil {
		return ""
	}

	return values.Get(key)
}

func parseCBORPath(path string) (string, string, bool) {
	if !strings.HasPrefix(path, "/service/") {
		return "", "", false
	}

	remaining := strings.TrimPrefix(path, "/service/")
	parts := strings.Split(remaining, "/operation/")

	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}

	return parts[0], parts[1], true
}

func inferRESTAction(serviceName, method, pattern string, req *http.Request) string {
	if serviceName != "s3" {
		return ""
	}

	q := req.URL.Query()

	switch pattern {
	case "/":
		if method == http.MethodGet {
			return "ListBuckets"
		}
	case "/{bucket}":
		return inferS3BucketAction(method, q)
	case "/{bucket}/{key...}":
		return inferS3ObjectAction(method, q, req.Header)
	}

	return ""
}

func inferS3BucketAction(method string, q url.Values) string {
	switch method {
	case http.MethodPut:
		return inferS3BucketPutAction(q)
	case http.MethodGet:
		return inferS3BucketGetAction(q)
	case http.MethodDelete:
		return "DeleteBucket"
	case http.MethodHead:
		return "HeadBucket"
	case http.MethodPost:
		if q.Has("delete") {
			return "DeleteObjects"
		}
	}

	return ""
}

func inferS3BucketPutAction(q url.Values) string {
	switch {
	case q.Has("versioning"):
		return "PutBucketVersioning"
	case q.Has("cors"):
		return "PutBucketCors"
	case q.Has("encryption"):
		return "PutBucketEncryption"
	default:
		return "CreateBucket"
	}
}

func inferS3BucketGetAction(q url.Values) string {
	switch {
	case q.Has("versioning"):
		return "GetBucketVersioning"
	case q.Has("cors"):
		return "GetBucketCors"
	case q.Has("encryption"):
		return "GetBucketEncryption"
	case q.Has("uploads"):
		return "ListMultipartUploads"
	default:
		return "ListObjects"
	}
}

func inferS3ObjectAction(method string, q url.Values, header http.Header) string {
	switch method {
	case http.MethodPut:
		switch {
		case q.Has("uploadId") && q.Has("partNumber"):
			return "UploadPart"
		case header.Get("x-amz-copy-source") != "":
			return "CopyObject"
		default:
			return "PutObject"
		}
	case http.MethodGet:
		return "GetObject"
	case http.MethodHead:
		return "HeadObject"
	case http.MethodDelete:
		return "DeleteObject"
	case http.MethodPost:
		switch {
		case q.Has("uploads"):
			return "CreateMultipartUpload"
		case q.Has("uploadId"):
			return "CompleteMultipartUpload"
		}
	}

	return ""
}

func inferResource(serviceName string, req *http.Request) string {
	if serviceName != "s3" {
		return ""
	}

	bucket := req.PathValue("bucket")
	key := req.PathValue("key")

	if bucket == "" {
		return ""
	}

	if key == "" {
		return bucket
	}

	return bucket + "/" + key
}
