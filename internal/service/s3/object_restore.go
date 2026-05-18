package s3

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"
)

// RestoreRequest is the XML body of POST /{bucket}/{key}?restore.
type RestoreRequest struct {
	XMLName              xml.Name          `xml:"RestoreRequest"`
	Days                 int               `xml:"Days,omitempty"`
	GlacierJobParameters *GlacierJobParams `xml:"GlacierJobParameters,omitempty"`
	Type                 string            `xml:"Type,omitempty"` // SELECT for query-in-place
	Tier                 string            `xml:"Tier,omitempty"` // Standard | Bulk | Expedited
	Description          string            `xml:"Description,omitempty"`
}

// GlacierJobParams is the legacy single-field shape that holds the
// retrieval Tier.
type GlacierJobParams struct {
	Tier string `xml:"Tier"`
}

// RestoreState tracks an in-progress / completed restore on a single
// object. kumo doesn't model storage classes, so the restore is
// considered "complete" the moment it's requested — but the state is
// persisted so HEAD/GET reports the right `x-amz-restore` header.
type RestoreState struct {
	OngoingRequest bool      `json:"ongoingRequest"`
	ExpiryDate     time.Time `json:"expiryDate"`
	Tier           string    `json:"tier"`
}

// RestoreObject handles POST /{bucket}/{key}?restore.
//
// Real S3 returns 202 Accepted (in-progress) for an Glacier object,
// 200 OK if a restore is already in progress and being extended, and
// 409 Conflict (RestoreAlreadyInProgress) under certain conditions.
//
// kumo accepts the request unconditionally:
//   - first restore on this key  → 202 Accepted
//   - subsequent restores        → 200 OK (extending the existing one)
func (s *Service) RestoreObject(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	key := r.PathValue("key")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeS3Error(w, r, "InvalidRequest", "Failed to read request body", http.StatusBadRequest)

		return
	}

	var req RestoreRequest
	if len(body) > 0 {
		if err := xml.Unmarshal(body, &req); err != nil {
			writeS3Error(w, r, "MalformedXML", fmt.Sprintf("RestoreRequest XML: %v", err), http.StatusBadRequest)

			return
		}
	}

	days := req.Days
	if days <= 0 {
		days = 1 // S3 default
	}

	tier := req.Tier
	if tier == "" && req.GlacierJobParameters != nil {
		tier = req.GlacierJobParameters.Tier
	}

	if tier == "" {
		tier = "Standard"
	}

	state := &RestoreState{
		OngoingRequest: false, // we declare the restore complete immediately
		ExpiryDate:     time.Now().Add(time.Duration(days) * 24 * time.Hour),
		Tier:           tier,
	}

	alreadyExisted, err := s.storage.PutObjectRestore(r.Context(), bucket, key, state)
	if err != nil {
		handleBucketLevelError(w, r, err)

		return
	}

	w.Header().Set("x-amz-restore-output-path", fmt.Sprintf("%s/%s", bucket, key))

	if alreadyExisted {
		w.WriteHeader(http.StatusOK)

		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// MemoryStorage hooks for object restore.

// PutObjectRestore stores the restore state. Returns alreadyExisted=true
// when the object already had restore state — caller uses that to
// distinguish initial 202 vs. extending-existing 200.
func (s *MemoryStorage) PutObjectRestore(_ context.Context, bucket, key string, state *RestoreState) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.Buckets[bucket]
	if !ok {
		return false, &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	if _, ok := b.Objects[key]; !ok {
		return false, &ObjectError{Code: "NoSuchKey", Message: "The specified key does not exist.", Key: key}
	}

	if b.ObjectRestores == nil {
		b.ObjectRestores = make(map[string]*RestoreState)
	}

	_, alreadyExisted := b.ObjectRestores[key]
	b.ObjectRestores[key] = state

	return alreadyExisted, nil
}
