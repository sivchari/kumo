package s3

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// maxPostFormMemory bounds in-memory buffering when parsing the multipart
	// form; larger uploads spill to temporary files.
	maxPostFormMemory = 32 << 20 // 32 MiB

	postFileField      = "file"
	postKeyField       = "key"
	postPolicyField    = "policy"
	postFilenameVar    = "${filename}"
	postRedirectField  = "success_action_redirect"
	postLegacyRedirect = "redirect"
	postStatusField    = "success_action_status"
	postStatusOK       = "200"
	postStatusCreated  = "201"
	postMetadataPrefix = "x-amz-meta-"
)

// PostObject handles POST /{bucket} with a multipart/form-data body, the
// browser-based ("presigned POST") upload form. The signed policy is supplied
// as form fields rather than as an Authorization header or query string.
// See https://docs.aws.amazon.com/AmazonS3/latest/API/RESTObjectPOST.html
func (s *Service) PostObject(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	if bucket == "" {
		writeS3Error(w, r, "InvalidBucketName", "The specified bucket is not valid.", http.StatusBadRequest)

		return
	}

	s.applyCORSHeaders(w, r, bucket)

	upload, ok := parsePostUpload(w, r)

	if r.MultipartForm != nil {
		defer func() { _ = r.MultipartForm.RemoveAll() }()
	}

	if !ok {
		return
	}

	defer func() { _ = upload.file.Close() }()

	obj, err := s.storage.PutObject(r.Context(), bucket, upload.key, upload.file, upload.metadata)
	if err != nil {
		var bucketErr *BucketError
		if errors.As(err, &bucketErr) {
			writeS3Error(w, r, bucketErr.Code, bucketErr.Message, http.StatusNotFound)

			return
		}

		writeS3Error(w, r, "InternalError", "Internal server error", http.StatusInternalServerError)

		return
	}

	if tags := parsePostTagging(r.FormValue("tagging")); len(tags) > 0 {
		_ = s.storage.PutObjectTagging(r.Context(), bucket, upload.key, tags)
	}

	w.Header().Set("ETag", obj.ETag)

	if obj.VersionID != "" {
		w.Header().Set("x-amz-version-id", obj.VersionID)
	}

	go s.emitObjectCreatedEvent(context.Background(), bucket, upload.key, obj.Size, obj.ETag)
	go s.emitSQSNotifications(context.Background(), bucket, upload.key, "s3:ObjectCreated:Post", obj.Size, obj.ETag)

	writePostObjectResponse(w, r, bucket, upload.key, obj)
}

// postUpload holds the data extracted from a POST Object multipart form. The
// caller owns closing file once it is done reading the object content.
type postUpload struct {
	key      string
	file     multipart.File
	metadata map[string]string
}

// parsePostUpload parses the multipart form, validates the policy, resolves the
// object key, and opens the uploaded file. On failure it writes an error
// response and returns ok=false. The caller is responsible for closing
// upload.file and for cleaning up r.MultipartForm.
func parsePostUpload(w http.ResponseWriter, r *http.Request) (postUpload, bool) {
	if err := r.ParseMultipartForm(maxPostFormMemory); err != nil {
		writeS3Error(w, r, "MalformedPOSTRequest", "The body of your POST request is not well-formed multipart/form-data.", http.StatusBadRequest)

		return postUpload{}, false
	}

	files := r.MultipartForm.File[postFileField]
	if len(files) == 0 {
		writeS3Error(w, r, "InvalidArgument", "POST requires exactly one file upload per request.", http.StatusBadRequest)

		return postUpload{}, false
	}

	fileHeader := files[0]

	key := r.FormValue(postKeyField)
	if key == "" {
		writeS3Error(w, r, "InvalidArgument", "Bucket POST must contain a field named 'key'. If it is specified, please check the order of the fields.", http.StatusBadRequest)

		return postUpload{}, false
	}

	// ${filename} is replaced with the uploaded file's original name.
	key = strings.ReplaceAll(key, postFilenameVar, fileHeader.Filename)

	if policy := r.FormValue(postPolicyField); policy != "" {
		if err := validatePostPolicy(policy); err != nil {
			writePostPolicyError(w, r, err)

			return postUpload{}, false
		}
	}

	file, err := fileHeader.Open()
	if err != nil {
		writeS3Error(w, r, "InternalError", "Internal server error", http.StatusInternalServerError)

		return postUpload{}, false
	}

	return postUpload{key: key, file: file, metadata: postObjectMetadata(r, fileHeader)}, true
}

// writePostPolicyError maps a policy validation error to an S3 error response.
func writePostPolicyError(w http.ResponseWriter, r *http.Request, err error) {
	var presignErr *PresignedURLError
	if errors.As(err, &presignErr) {
		writeS3Error(w, r, presignErr.Code, presignErr.Message, http.StatusForbidden)

		return
	}

	writeS3Error(w, r, "InternalError", "Internal server error", http.StatusInternalServerError)
}

// validatePostPolicy decodes the base64 POST policy document and rejects the
// request if the policy has expired. Like presigned URLs, kumo validates the
// expiration only and does not recompute the HMAC signature.
func validatePostPolicy(encoded string) error {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return &PresignedURLError{Code: "InvalidPolicyDocument", Message: "The content of the form does not meet the conditions specified in the policy document."}
	}

	var doc struct {
		Expiration string `json:"expiration"`
	}

	if err := json.Unmarshal(raw, &doc); err != nil {
		return &PresignedURLError{Code: "InvalidPolicyDocument", Message: "Invalid Policy: Invalid JSON."}
	}

	if doc.Expiration == "" {
		return nil
	}

	expiration, err := parsePolicyExpiration(doc.Expiration)
	if err != nil {
		return &PresignedURLError{Code: "InvalidPolicyDocument", Message: "Invalid Policy: Invalid expiration."}
	}

	if time.Now().After(expiration) {
		return &PresignedURLError{Code: "AccessDenied", Message: "Invalid according to Policy: Policy expired."}
	}

	return nil
}

// parsePolicyExpiration parses the policy expiration timestamp, accepting both
// the RFC3339 form the AWS SDK emits and the millisecond ISO8601 variant.
func parsePolicyExpiration(value string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05.000Z", "2006-01-02T15:04:05Z"} {
		if t, err := time.Parse(layout, value); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unrecognized expiration format: %q", value)
}

// postObjectMetadata extracts the object metadata carried by the form fields:
// an explicit Content-Type field (falling back to the file part's own type)
// and any x-amz-meta-* user metadata fields.
func postObjectMetadata(r *http.Request, fileHeader *multipart.FileHeader) map[string]string {
	metadata := make(map[string]string)

	if ct := r.FormValue(contentTypeHeader); ct != "" {
		metadata[contentTypeHeader] = ct
	} else if ct := fileHeader.Header.Get(contentTypeHeader); ct != "" {
		metadata[contentTypeHeader] = ct
	}

	for name, values := range r.MultipartForm.Value {
		if len(values) == 0 {
			continue
		}

		if metaKey, found := strings.CutPrefix(strings.ToLower(name), postMetadataPrefix); found {
			metadata[metaKey] = values[0]
		}
	}

	return metadata
}

// parsePostTagging parses the optional "tagging" form field, an XML <Tagging>
// document, into a tag map. Malformed input is ignored.
func parsePostTagging(raw string) map[string]string {
	if raw == "" {
		return nil
	}

	var doc struct {
		TagSet []struct {
			Key   string `xml:"Key"`
			Value string `xml:"Value"`
		} `xml:"TagSet>Tag"`
	}

	if err := xml.Unmarshal([]byte(raw), &doc); err != nil {
		return nil
	}

	tags := make(map[string]string, len(doc.TagSet))
	for _, tag := range doc.TagSet {
		tags[tag.Key] = tag.Value
	}

	return tags
}

// writePostObjectResponse writes the POST Object response according to the
// success_action_redirect / success_action_status fields, defaulting to 204.
func writePostObjectResponse(w http.ResponseWriter, r *http.Request, bucket, key string, obj *Object) {
	location := fmt.Sprintf("http://%s/%s/%s", r.Host, bucket, key)

	redirect := r.FormValue(postRedirectField)
	if redirect == "" {
		redirect = r.FormValue(postLegacyRedirect)
	}

	if redirect != "" {
		if u, err := url.Parse(redirect); err == nil {
			q := u.Query()
			q.Set("bucket", bucket)
			q.Set("key", key)
			q.Set("etag", obj.ETag)
			u.RawQuery = q.Encode()

			w.Header().Set("Location", u.String())
			w.WriteHeader(http.StatusSeeOther)

			return
		}
	}

	switch r.FormValue(postStatusField) {
	case postStatusOK:
		w.WriteHeader(http.StatusOK)
	case postStatusCreated:
		w.Header().Set(contentTypeHeader, "application/xml")
		w.WriteHeader(http.StatusCreated)

		_, _ = io.WriteString(w, xmlHeader)
		_ = xml.NewEncoder(w).Encode(PostResponse{
			Location: location,
			Bucket:   bucket,
			Key:      key,
			ETag:     obj.ETag,
		})
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
