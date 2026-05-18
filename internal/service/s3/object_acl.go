package s3

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// ObjectACL is the in-memory model of an object's Access Control List.
// kumo doesn't enforce ACL at request time (no permission check on
// GetObject / PutObject), but persisting the ACL means tools that
// PutObjectAcl + GetObjectAcl in roundtrip — Cloudtrail-style audit
// pipelines, S3 compliance scans, terraform `aws_s3_bucket_acl`
// resources — see consistent state.
type ObjectACL struct {
	CannedACL string
	OwnerID   string
	OwnerName string
	Grants    []ACLGrant
}

// ACLGrant is one entry in an ObjectACL's grant list.
type ACLGrant struct {
	GranteeType string // "CanonicalUser" or "Group"
	GranteeID   string // CanonicalUser ID
	GranteeURI  string // Group URI
	DisplayName string
	Permission  string // "FULL_CONTROL" | "READ" | "WRITE" | "READ_ACP" | "WRITE_ACP"
}

// XML wire types — match S3's AccessControlPolicy schema.

// AccessControlPolicy is the XML body of GET/PUT object?acl.
type AccessControlPolicy struct {
	XMLName xml.Name              `xml:"AccessControlPolicy"`
	Xmlns   string                `xml:"xmlns,attr,omitempty"`
	Owner   AccessControlOwner    `xml:"Owner"`
	ACL     AccessControlListBody `xml:"AccessControlList"`
}

// AccessControlOwner is the owner of an object in an AccessControlPolicy.
type AccessControlOwner struct {
	ID          string `xml:"ID"`
	DisplayName string `xml:"DisplayName,omitempty"`
}

// AccessControlListBody is the body of the AccessControlList element.
type AccessControlListBody struct {
	Grants []AccessControlGrant `xml:"Grant"`
}

// AccessControlGrant is one Grant entry in an AccessControlList.
type AccessControlGrant struct {
	XMLName    xml.Name             `xml:"Grant"`
	Grantee    AccessControlGrantee `xml:"Grantee"`
	Permission string               `xml:"Permission"`
}

// AccessControlGrantee is the Grantee element of a Grant.
type AccessControlGrantee struct {
	XSI         string `xml:"xmlns:xsi,attr,omitempty"`
	Type        string `xml:"xsi:type,attr"`
	ID          string `xml:"ID,omitempty"`
	URI         string `xml:"URI,omitempty"`
	DisplayName string `xml:"DisplayName,omitempty"`
}

// PutObjectACL handles PUT /{bucket}/{key}?acl.
//
// The grant list comes from one of two sources, mutually exclusive:
//   - request body XML (AccessControlPolicy)
//   - `x-amz-acl` header (canned ACL: private, public-read, …)
//
// kumo accepts both; precedence matches real S3 — if both arrive the
// XML body wins.
func (s *Service) PutObjectACL(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	key := r.PathValue("key")

	if bucket == "" || key == "" {
		writeS3Error(w, r, "InvalidArgument", "Invalid bucket or key", http.StatusBadRequest)

		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeS3Error(w, r, "InvalidRequest", "Failed to read body", http.StatusBadRequest)

		return
	}

	acl, err := decodeObjectACL(body, r.Header.Get("X-Amz-Acl"))
	if err != nil {
		writeS3Error(w, r, "MalformedACLError", err.Error(), http.StatusBadRequest)

		return
	}

	if err := s.storage.PutObjectACL(r.Context(), bucket, key, acl); err != nil {
		handleObjectACLError(w, r, err)

		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetObjectACL handles GET /{bucket}/{key}?acl.
func (s *Service) GetObjectACL(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	key := r.PathValue("key")

	acl, err := s.storage.GetObjectACL(r.Context(), bucket, key)
	if err != nil {
		handleObjectACLError(w, r, err)

		return
	}

	writeXMLResponse(w, encodeObjectACL(acl))
}

// decodeObjectACL turns a (body, cannedACL) pair into an ObjectACL.
// Empty body + empty header → returns the default canned `private`
// ACL so PutObjectAcl with no payload is still meaningful.
func decodeObjectACL(body []byte, cannedHeader string) (*ObjectACL, error) {
	if len(body) > 0 {
		var p AccessControlPolicy
		if err := xml.Unmarshal(body, &p); err != nil {
			return nil, fmt.Errorf("parse AccessControlPolicy XML: %w", err)
		}

		acl := &ObjectACL{
			OwnerID:   p.Owner.ID,
			OwnerName: p.Owner.DisplayName,
			Grants:    make([]ACLGrant, 0, len(p.ACL.Grants)),
		}

		for i := range p.ACL.Grants {
			g := &p.ACL.Grants[i]
			acl.Grants = append(acl.Grants, ACLGrant{
				GranteeType: g.Grantee.Type,
				GranteeID:   g.Grantee.ID,
				GranteeURI:  g.Grantee.URI,
				DisplayName: g.Grantee.DisplayName,
				Permission:  g.Permission,
			})
		}

		return acl, nil
	}

	canned := cannedHeader
	if canned == "" {
		canned = "private"
	}

	return &ObjectACL{CannedACL: canned, OwnerID: "owner-id", OwnerName: "owner"}, nil
}

// encodeObjectACL renders an ObjectACL back to AccessControlPolicy
// XML. Canned ACLs are expanded to a synthetic single grant for the
// owner (FULL_CONTROL) so the response shape is always the same — that
// matches what real S3 emits when you GetObjectAcl on an object that
// was set via `x-amz-acl: private`.
func encodeObjectACL(acl *ObjectACL) *AccessControlPolicy {
	p := &AccessControlPolicy{
		Xmlns: s3Namespace,
		Owner: AccessControlOwner{ID: acl.OwnerID, DisplayName: acl.OwnerName},
	}

	if len(acl.Grants) == 0 {
		p.ACL.Grants = []AccessControlGrant{{
			Grantee: AccessControlGrantee{
				XSI:         "http://www.w3.org/2001/XMLSchema-instance",
				Type:        "CanonicalUser",
				ID:          acl.OwnerID,
				DisplayName: acl.OwnerName,
			},
			Permission: "FULL_CONTROL",
		}}

		return p
	}

	p.ACL.Grants = make([]AccessControlGrant, len(acl.Grants))
	for i, g := range acl.Grants {
		p.ACL.Grants[i] = AccessControlGrant{
			Grantee: AccessControlGrantee{
				XSI:         "http://www.w3.org/2001/XMLSchema-instance",
				Type:        g.GranteeType,
				ID:          g.GranteeID,
				URI:         g.GranteeURI,
				DisplayName: g.DisplayName,
			},
			Permission: g.Permission,
		}
	}

	return p
}

func handleObjectACLError(w http.ResponseWriter, r *http.Request, err error) {
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

// MemoryStorage hooks for object ACLs.

// PutObjectACL stores an ACL alongside the object.
func (s *MemoryStorage) PutObjectACL(_ context.Context, bucket, key string, acl *ObjectACL) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.Buckets[bucket]
	if !ok {
		return &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	if _, ok := b.Objects[key]; !ok {
		return &ObjectError{Code: "NoSuchKey", Message: "The specified key does not exist.", Key: key}
	}

	if b.ObjectACLs == nil {
		b.ObjectACLs = make(map[string]*ObjectACL)
	}

	b.ObjectACLs[key] = acl

	return nil
}

// GetObjectACL returns the stored ACL for an object, or a default
// owner-FULL_CONTROL ACL when none was set.
func (s *MemoryStorage) GetObjectACL(_ context.Context, bucket, key string) (*ObjectACL, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	b, ok := s.Buckets[bucket]
	if !ok {
		return nil, &BucketError{Code: "NoSuchBucket", Message: "The specified bucket does not exist", BucketName: bucket}
	}

	if _, ok := b.Objects[key]; !ok {
		return nil, &ObjectError{Code: "NoSuchKey", Message: "The specified key does not exist.", Key: key}
	}

	if acl, ok := b.ObjectACLs[key]; ok {
		return acl, nil
	}

	return &ObjectACL{CannedACL: "private", OwnerID: "owner-id", OwnerName: "owner"}, nil
}
