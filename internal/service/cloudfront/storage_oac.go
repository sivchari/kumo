package cloudfront

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"

	"github.com/sivchari/kumo/internal/vhost"
)

const oacIDLength = 13

// ensureOACInit lazily initialises the origin access control map, which a
// snapshot written before the feature existed does not contain.
func (s *MemoryStorage) ensureOACInit() {
	if s.OriginAccessControls == nil {
		s.OriginAccessControls = make(map[string]*OriginAccessControl)
	}
}

// CreateOriginAccessControl stores a new origin access control. Names are
// unique per account (OriginAccessControlAlreadyExists).
func (s *MemoryStorage) CreateOriginAccessControl(_ context.Context, cfg *OriginAccessControlConfig) (*OriginAccessControl, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensureOACInit()

	if err := validateOriginAccessControlConfig(cfg); err != nil {
		return nil, err
	}

	if err := s.oacNameAvailableLocked(cfg.Name, ""); err != nil {
		return nil, err
	}

	oac := &OriginAccessControl{ID: generateOriginAccessControlID(), ETag: generateETag(), Config: *cfg}
	s.OriginAccessControls[oac.ID] = oac

	s.saveLocked()

	return oac.clone(), nil
}

// GetOriginAccessControl returns a copy of the origin access control.
func (s *MemoryStorage) GetOriginAccessControl(_ context.Context, id string) (*OriginAccessControl, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	oac, err := s.oacLocked(id)
	if err != nil {
		return nil, err
	}

	return oac.clone(), nil
}

// ListOriginAccessControls returns the origin access controls sorted by id,
// starting after marker and limited to maxItems; nextMarker is the last id
// returned when more remain.
func (s *MemoryStorage) ListOriginAccessControls(_ context.Context, marker string, maxItems int) ([]*OriginAccessControl, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if maxItems <= 0 {
		maxItems = oacListMaxItems
	}

	all := make([]*OriginAccessControl, 0, len(s.OriginAccessControls))

	for _, oac := range s.OriginAccessControls {
		if oac.ID > marker {
			all = append(all, oac.clone())
		}
	}

	sortByID(all, func(o *OriginAccessControl) string { return o.ID })

	if len(all) <= maxItems {
		return all, "", nil
	}

	page := all[:maxItems]

	return page, page[len(page)-1].ID, nil
}

// UpdateOriginAccessControl replaces the configuration when ifMatch is the
// current ETag.
func (s *MemoryStorage) UpdateOriginAccessControl(_ context.Context, id string, cfg *OriginAccessControlConfig, ifMatch string) (*OriginAccessControl, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	oac, err := s.oacForWriteLocked(id, ifMatch)
	if err != nil {
		return nil, err
	}

	if err := validateOriginAccessControlConfig(cfg); err != nil {
		return nil, err
	}

	if err := s.oacNameAvailableLocked(cfg.Name, id); err != nil {
		return nil, err
	}

	oac.Config = *cfg
	oac.ETag = generateETag()

	s.saveLocked()

	return oac.clone(), nil
}

// DeleteOriginAccessControl removes the origin access control when ifMatch
// is the current ETag and no distribution origin references it.
func (s *MemoryStorage) DeleteOriginAccessControl(_ context.Context, id, ifMatch string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.oacForWriteLocked(id, ifMatch); err != nil {
		return err
	}

	for _, d := range s.Distributions {
		if d.DistributionConfig == nil || d.DistributionConfig.Origins == nil {
			continue
		}

		for _, o := range d.DistributionConfig.Origins.Items {
			if o.OriginAccessControlID == id {
				return &Error{Code: errOriginAccessControlInUse, Message: fmt.Sprintf("Cannot delete the origin access control because it's in use by distribution %s.", d.ID)}
			}
		}
	}

	delete(s.OriginAccessControls, id)

	s.saveLocked()

	return nil
}

func (s *MemoryStorage) oacLocked(id string) (*OriginAccessControl, error) {
	oac, ok := s.OriginAccessControls[id]
	if !ok {
		return nil, &Error{Code: errNoSuchOriginAccessControl, Message: "The origin access control does not exist."}
	}

	return oac, nil
}

// oacForWriteLocked applies the If-Match protocol: the header is mandatory
// (InvalidIfMatchVersion) and must carry the current ETag (PreconditionFailed).
func (s *MemoryStorage) oacForWriteLocked(id, ifMatch string) (*OriginAccessControl, error) {
	if ifMatch == "" {
		return nil, &Error{Code: errInvalidIfMatchVersion, Message: "The If-Match version is missing or not valid for the resource."}
	}

	oac, err := s.oacLocked(id)
	if err != nil {
		return nil, err
	}

	if oac.ETag != ifMatch {
		return nil, &Error{Code: errPreconditionFailed, Message: "The precondition in one or more of the request fields evaluated to false."}
	}

	return oac, nil
}

// oacNameAvailableLocked rejects a name already used by another origin access control.
func (s *MemoryStorage) oacNameAvailableLocked(name, exceptID string) error {
	for _, other := range s.OriginAccessControls {
		if other.ID != exceptID && other.Config.Name == name {
			return &Error{Code: errOriginAccessControlAlreadyExists, Message: "An origin access control with the specified parameters already exists."}
		}
	}

	return nil
}

// validateOriginAccessControlConfig enforces the documented value sets.
func validateOriginAccessControlConfig(cfg *OriginAccessControlConfig) error {
	switch {
	case cfg.Name == "" || len(cfg.Name) > oacNameMaxLength:
		return invalidArgument(fmt.Sprintf("Name must be between 1 and %d characters.", oacNameMaxLength))
	case !slices.Contains([]string{oacOriginTypeS3, oacOriginTypeMediaStore, oacOriginTypeMediaPackageV2, oacOriginTypeLambda}, cfg.OriginAccessControlOriginType):
		return invalidArgument("OriginAccessControlOriginType must be one of s3, mediastore, mediapackagev2, lambda.")
	case !slices.Contains([]string{oacSigningAlways, oacSigningNever, oacSigningNoOverride}, cfg.SigningBehavior):
		return invalidArgument("SigningBehavior must be one of always, never, no-override.")
	case cfg.SigningProtocol != oacSigningProtocolSigV4:
		return invalidArgument("SigningProtocol must be sigv4.")
	}

	return nil
}

func invalidArgument(message string) error {
	return &Error{Code: errInvalidArgument, Message: message}
}

// validateOriginAccessLocked checks the origin access controls a distribution
// references: they must exist, an origin cannot combine one with an origin
// access identity, and s3 / lambda controls only fit matching origin domains.
// The caller holds the lock.
func (s *MemoryStorage) validateOriginAccessLocked(config *DistributionConfig) error {
	if config.Origins == nil {
		return nil
	}

	for i := range config.Origins.Items {
		o := &config.Origins.Items[i]
		if o.OriginAccessControlID == "" {
			continue
		}

		oac, ok := s.OriginAccessControls[o.OriginAccessControlID]
		if !ok {
			return &Error{Code: errInvalidOriginAccessControl, Message: "The origin access control is not valid."}
		}

		if o.S3OriginConfig != nil && o.S3OriginConfig.OriginAccessIdentity != "" {
			return &Error{Code: errIllegalOriginAccessConfiguration, Message: "An origin cannot contain both an origin access control (OAC) and an origin access identity (OAI)."}
		}

		if !originDomainSupportsOAC(oac.Config.OriginAccessControlOriginType, o.DomainName) {
			return &Error{Code: errInvalidDomainNameForOriginAccessControl, Message: "An origin access control is associated with an origin whose domain name is not supported."}
		}
	}

	return nil
}

// originDomainSupportsOAC reports whether an origin domain fits the origin
// access control type: S3 endpoints that support OAC for s3, Lambda function
// URL hosts for lambda. mediastore and mediapackagev2 are not checked (kumo
// has no such origins).
func originDomainSupportsOAC(originType, domain string) bool {
	switch originType {
	case oacOriginTypeS3:
		return isS3OACDomain(domain)
	case oacOriginTypeLambda:
		_, ok := vhost.FunctionURLID(domain)

		return ok
	}

	return true
}

// isS3OACDomain recognises the S3 endpoints CloudFront can sign for with an
// s3 origin access control: the REST endpoint (`<bucket>.s3.amazonaws.com`,
// `<bucket>.s3.<region>.amazonaws.com`, legacy `<bucket>.s3-<region>...`),
// dualstack, access points (`s3-accesspoint`) and multi-region access points
// (`<alias>.accesspoint.s3-global.amazonaws.com`), in the standard and China
// partitions. Website (`s3-website-*`) and transfer acceleration endpoints
// are custom origins on CloudFront and are rejected.
func isS3OACDomain(domain string) bool {
	host := strings.ToLower(domain)
	if !strings.HasSuffix(host, ".amazonaws.com") && !strings.HasSuffix(host, ".amazonaws.com.cn") {
		return false
	}

	labels := strings.Split(host, ".")
	if len(labels) < 3 || labels[0] == "" {
		return false
	}

	// The first label is the bucket or access point alias; the service label
	// follows it (possibly after "accesspoint" for a multi-region access point).
	for _, label := range labels[1:] {
		switch {
		case strings.HasPrefix(label, "s3-website"), label == "s3-accelerate":
			return false
		case label == "s3", label == "s3-accesspoint", label == "s3-global", strings.HasPrefix(label, "s3-"):
			return true
		}
	}

	return false
}

// generateOriginAccessControlID returns an id of the AWS shape: E followed
// by 13 upper-case alphanumerics.
func generateOriginAccessControlID() string {
	raw := make([]byte, oacIDLength)
	_, _ = rand.Read(raw)

	return "E" + strings.ToUpper(hex.EncodeToString(raw))[:oacIDLength]
}

func (o *OriginAccessControl) clone() *OriginAccessControl {
	out := *o

	return &out
}
