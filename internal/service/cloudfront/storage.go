package cloudfront

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

// Storage defines the CloudFront storage interface.
type Storage interface {
	CreateDistribution(ctx context.Context, config *CreateDistributionRequest) (*Distribution, error)
	GetDistribution(ctx context.Context, id string) (*Distribution, error)
	ListDistributions(ctx context.Context, marker string, maxItems int) ([]*Distribution, string, error)
	UpdateDistribution(ctx context.Context, id string, config *CreateDistributionRequest, etag string) (*Distribution, error)
	DeleteDistribution(ctx context.Context, id string, etag string) error
	CreateInvalidation(ctx context.Context, distributionID string, batch *CreateInvalidationRequest) (*Invalidation, error)
	GetInvalidation(ctx context.Context, distributionID, invalidationID string) (*Invalidation, error)
	ListInvalidations(ctx context.Context, distributionID, marker string, maxItems int) ([]*Invalidation, string, error)

	// Tagging operations, keyed by distribution ARN.
	TagResource(ctx context.Context, arn string, tags map[string]string) error
	UntagResource(ctx context.Context, arn string, keys []string) error
	ListTags(ctx context.Context, arn string) (map[string]string, error)

	// Signed URL building blocks.
	CreatePublicKey(ctx context.Context, cfg *PublicKeyConfig) (*PublicKey, error)
	GetPublicKey(ctx context.Context, id string) (*PublicKey, error)
	ListPublicKeys(ctx context.Context) []*PublicKey
	DeletePublicKey(ctx context.Context, id string) error

	CreateKeyGroup(ctx context.Context, cfg *KeyGroupConfig) (*KeyGroup, error)
	GetKeyGroup(ctx context.Context, id string) (*KeyGroup, error)
	ListKeyGroups(ctx context.Context) []*KeyGroup
	DeleteKeyGroup(ctx context.Context, id string) error
}

// Option is a configuration option for MemoryStorage.
type Option func(*MemoryStorage)

// WithDataDir enables persistent storage in the specified directory.
func WithDataDir(dir string) Option {
	return func(s *MemoryStorage) {
		s.dataDir = dir
	}
}

// Compile-time interface checks.
var (
	_ json.Marshaler   = (*MemoryStorage)(nil)
	_ json.Unmarshaler = (*MemoryStorage)(nil)
)

// MemoryStorage implements Storage with in-memory data.
type MemoryStorage struct {
	mu            sync.RWMutex                        `json:"-"`
	Distributions map[string]*Distribution            `json:"distributions"`
	Invalidations map[string]map[string]*Invalidation `json:"invalidations"` // distributionID -> invalidationID -> Invalidation
	signing       signingStore
	dataDir       string
}

// NewMemoryStorage creates a new memory storage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		Distributions: make(map[string]*Distribution),
		Invalidations: make(map[string]map[string]*Invalidation),
		signing: signingStore{
			PublicKeys: make(map[string]*PublicKey),
			KeyGroups:  make(map[string]*KeyGroup),
		},
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "cloudfront", s)
	}

	return s
}

// MarshalJSON serializes the storage state to JSON.
func (s *MemoryStorage) MarshalJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type Alias MemoryStorage

	data, err := json.Marshal(&struct{ *Alias }{Alias: (*Alias)(s)})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}

	return data, nil
}

// UnmarshalJSON restores the storage state from JSON.
func (s *MemoryStorage) UnmarshalJSON(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	type Alias MemoryStorage

	aux := &struct{ *Alias }{Alias: (*Alias)(s)}

	if err := json.Unmarshal(data, aux); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	if s.Distributions == nil {
		s.Distributions = make(map[string]*Distribution)
	}

	if s.Invalidations == nil {
		s.Invalidations = make(map[string]map[string]*Invalidation)
	}

	s.ensureSigningInit()

	return nil
}

// saveLocked persists the current state to disk while the caller holds the lock.
func (s *MemoryStorage) saveLocked() {
	if s.dataDir == "" {
		return
	}

	storage.ScheduleSave(s.dataDir, "cloudfront", s.MarshalJSON)
}

// Close saves the storage state to disk if persistence is enabled.
func (s *MemoryStorage) Close() error {
	if s.dataDir == "" {
		return nil
	}

	if err := storage.Save(s.dataDir, "cloudfront", s); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// Distribution deployment statuses.
const (
	statusInProgress = "InProgress"
	statusDeployed   = "Deployed"
)

// Error represents a CloudFront error.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	return e.Message
}

// CreateDistribution creates a new distribution.
func (s *MemoryStorage) CreateDistribution(_ context.Context, config *CreateDistributionRequest) (*Distribution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicate caller reference.
	for _, d := range s.Distributions {
		if d.DistributionConfig != nil && d.DistributionConfig.CallerReference == config.CallerReference {
			return nil, &Error{
				Code:    errDistributionAlreadyExists,
				Message: fmt.Sprintf("A distribution with caller reference %s already exists", config.CallerReference),
			}
		}
	}

	// Generate distribution ID.
	id := generateDistributionID()
	etag := generateETag()
	now := time.Now()

	dist := &Distribution{
		ID:               id,
		ARN:              fmt.Sprintf("arn:aws:cloudfront::000000000000:distribution/%s", id),
		Status:           statusInProgress,
		LastModifiedTime: now,
		DomainName:       fmt.Sprintf("%s.cloudfront.net", id),
		ETag:             etag,
		DistributionConfig: &DistributionConfig{
			CallerReference:      config.CallerReference,
			Comment:              config.Comment,
			Enabled:              config.Enabled,
			PriceClass:           defaultString(config.PriceClass, "PriceClass_All"),
			DefaultRootObject:    config.DefaultRootObject,
			HTTPVersion:          defaultString(config.HTTPVersion, "http2"),
			IsIPV6Enabled:        config.IsIPV6Enabled,
			Origins:              convertOriginsFromXML(config.Origins),
			DefaultCacheBehavior: convertDefaultCacheBehaviorFromXML(config.DefaultCacheBehavior),
			Aliases:              convertAliasesFromXML(config.Aliases),
			ViewerCertificate:    convertViewerCertificateFromXML(config.ViewerCertificate),
		},
		ActiveTrustedSigners:   &ActiveTrustedSigners{Enabled: false, Quantity: 0},
		ActiveTrustedKeyGroups: &ActiveTrustedKeyGroups{Enabled: false, Quantity: 0},
	}

	s.Distributions[id] = dist

	s.saveLocked()

	return dist, nil
}

// GetDistribution retrieves a distribution by ID.
//
// A newly created distribution starts as InProgress; the first read marks it
// Deployed so that Terraform's wait_for_deployment poller completes instead of
// timing out.
func (s *MemoryStorage) GetDistribution(_ context.Context, id string) (*Distribution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dist, exists := s.Distributions[id]
	if !exists {
		return nil, &Error{
			Code:    errDistributionNotFound,
			Message: fmt.Sprintf("The distribution with id %s does not exist", id),
		}
	}

	if dist.Status == statusInProgress {
		dist.Status = statusDeployed

		s.saveLocked()
	}

	return dist, nil
}

// ListDistributions lists all distributions.
func (s *MemoryStorage) ListDistributions(_ context.Context, marker string, maxItems int) ([]*Distribution, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if maxItems <= 0 {
		maxItems = 100
	}

	dists := make([]*Distribution, 0, len(s.Distributions))
	for _, d := range s.Distributions {
		dists = append(dists, d)
	}

	// Sort by ID for consistent ordering.
	sortDistributionsByID(dists)

	// Apply marker-based pagination.
	startIdx := 0

	if marker != "" {
		for i, d := range dists {
			if d.ID == marker {
				startIdx = i + 1

				break
			}
		}
	}

	// Slice the results.
	endIdx := min(startIdx+maxItems, len(dists))

	result := dists[startIdx:endIdx]

	// Determine next marker.
	var nextMarker string
	if endIdx < len(dists) {
		nextMarker = dists[endIdx-1].ID
	}

	return result, nextMarker, nil
}

// UpdateDistribution updates a distribution.
func (s *MemoryStorage) UpdateDistribution(_ context.Context, id string, config *CreateDistributionRequest, etag string) (*Distribution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dist, exists := s.Distributions[id]
	if !exists {
		return nil, &Error{
			Code:    errDistributionNotFound,
			Message: fmt.Sprintf("The distribution with id %s does not exist", id),
		}
	}

	// Validate ETag.
	if dist.ETag != etag {
		return nil, &Error{
			Code:    errInvalidIfMatchVersion,
			Message: "The If-Match version is missing or not valid for the resource",
		}
	}

	// Update distribution.
	newETag := generateETag()
	dist.ETag = newETag
	dist.LastModifiedTime = time.Now()
	dist.Status = statusInProgress
	dist.DistributionConfig = &DistributionConfig{
		CallerReference:      config.CallerReference,
		Comment:              config.Comment,
		Enabled:              config.Enabled,
		PriceClass:           defaultString(config.PriceClass, "PriceClass_All"),
		DefaultRootObject:    config.DefaultRootObject,
		HTTPVersion:          defaultString(config.HTTPVersion, "http2"),
		IsIPV6Enabled:        config.IsIPV6Enabled,
		Origins:              convertOriginsFromXML(config.Origins),
		DefaultCacheBehavior: convertDefaultCacheBehaviorFromXML(config.DefaultCacheBehavior),
		Aliases:              convertAliasesFromXML(config.Aliases),
		ViewerCertificate:    convertViewerCertificateFromXML(config.ViewerCertificate),
	}

	s.saveLocked()

	return dist, nil
}

// DeleteDistribution deletes a distribution.
func (s *MemoryStorage) DeleteDistribution(_ context.Context, id, etag string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dist, exists := s.Distributions[id]
	if !exists {
		return &Error{
			Code:    errDistributionNotFound,
			Message: fmt.Sprintf("The distribution with id %s does not exist", id),
		}
	}

	// Validate ETag.
	if dist.ETag != etag {
		return &Error{
			Code:    errInvalidIfMatchVersion,
			Message: "The If-Match version is missing or not valid for the resource",
		}
	}

	delete(s.Distributions, id)
	delete(s.Invalidations, id)

	s.saveLocked()

	return nil
}

// CreateInvalidation creates a new invalidation.
func (s *MemoryStorage) CreateInvalidation(_ context.Context, distributionID string, batch *CreateInvalidationRequest) (*Invalidation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if distribution exists.
	if _, exists := s.Distributions[distributionID]; !exists {
		return nil, &Error{
			Code:    errDistributionNotFound,
			Message: fmt.Sprintf("The distribution with id %s does not exist", distributionID),
		}
	}

	// Generate invalidation ID.
	id := generateInvalidationID()
	now := time.Now()

	inv := &Invalidation{
		ID:         id,
		Status:     "InProgress",
		CreateTime: now,
		InvalidationBatch: &InvalidationBatch{
			CallerReference: batch.CallerReference,
			Paths:           convertPathsFromXML(batch.Paths),
		},
	}

	// Store invalidation.
	if s.Invalidations[distributionID] == nil {
		s.Invalidations[distributionID] = make(map[string]*Invalidation)
	}

	s.Invalidations[distributionID][id] = inv

	s.saveLocked()

	return inv, nil
}

// GetInvalidation retrieves an invalidation.
func (s *MemoryStorage) GetInvalidation(_ context.Context, distributionID, invalidationID string) (*Invalidation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check if distribution exists.
	if _, exists := s.Distributions[distributionID]; !exists {
		return nil, &Error{
			Code:    errDistributionNotFound,
			Message: fmt.Sprintf("The distribution with id %s does not exist", distributionID),
		}
	}

	invMap, exists := s.Invalidations[distributionID]
	if !exists {
		return nil, &Error{
			Code:    errNoSuchInvalidation,
			Message: fmt.Sprintf("The invalidation with id %s does not exist", invalidationID),
		}
	}

	inv, exists := invMap[invalidationID]
	if !exists {
		return nil, &Error{
			Code:    errNoSuchInvalidation,
			Message: fmt.Sprintf("The invalidation with id %s does not exist", invalidationID),
		}
	}

	return inv, nil
}

// ListInvalidations lists invalidations for a distribution.
func (s *MemoryStorage) ListInvalidations(_ context.Context, distributionID, marker string, maxItems int) ([]*Invalidation, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check if distribution exists.
	if _, exists := s.Distributions[distributionID]; !exists {
		return nil, "", &Error{
			Code:    errDistributionNotFound,
			Message: fmt.Sprintf("The distribution with id %s does not exist", distributionID),
		}
	}

	if maxItems <= 0 {
		maxItems = 100
	}

	invMap := s.Invalidations[distributionID]
	if invMap == nil {
		return []*Invalidation{}, "", nil
	}

	var invs []*Invalidation
	for _, inv := range invMap {
		invs = append(invs, inv)
	}

	// Sort by ID for consistent ordering.
	sortInvalidationsByID(invs)

	// Apply marker-based pagination.
	startIdx := 0

	if marker != "" {
		for i, inv := range invs {
			if inv.ID == marker {
				startIdx = i + 1

				break
			}
		}
	}

	// Slice the results.
	endIdx := min(startIdx+maxItems, len(invs))

	result := invs[startIdx:endIdx]

	// Determine next marker.
	var nextMarker string
	if endIdx < len(invs) {
		nextMarker = invs[endIdx-1].ID
	}

	return result, nextMarker, nil
}

// findByARN returns the distribution matching the given ARN. The caller must
// hold s.mu.
func (s *MemoryStorage) findByARN(arn string) (*Distribution, bool) {
	for _, d := range s.Distributions {
		if d.ARN == arn {
			return d, true
		}
	}

	return nil, false
}

// TagResource adds or overwrites tags on the distribution identified by ARN.
func (s *MemoryStorage) TagResource(_ context.Context, arn string, tags map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dist, ok := s.findByARN(arn)
	if !ok {
		return &Error{
			Code:    errDistributionNotFound,
			Message: fmt.Sprintf("The resource %s does not exist", arn),
		}
	}

	if dist.Tags == nil {
		dist.Tags = make(map[string]string)
	}

	maps.Copy(dist.Tags, tags)

	s.saveLocked()

	return nil
}

// UntagResource removes the given tag keys from the distribution identified by ARN.
func (s *MemoryStorage) UntagResource(_ context.Context, arn string, keys []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dist, ok := s.findByARN(arn)
	if !ok {
		return &Error{
			Code:    errDistributionNotFound,
			Message: fmt.Sprintf("The resource %s does not exist", arn),
		}
	}

	for _, k := range keys {
		delete(dist.Tags, k)
	}

	s.saveLocked()

	return nil
}

// ListTags returns a copy of the tags on the distribution identified by ARN.
func (s *MemoryStorage) ListTags(_ context.Context, arn string) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dist, ok := s.findByARN(arn)
	if !ok {
		return nil, &Error{
			Code:    errDistributionNotFound,
			Message: fmt.Sprintf("The resource %s does not exist", arn),
		}
	}

	out := make(map[string]string, len(dist.Tags))
	maps.Copy(out, dist.Tags)

	return out, nil
}

// Helper functions.

func generateDistributionID() string {
	return "E" + uuid.New().String()[:13]
}

func generateInvalidationID() string {
	return "I" + uuid.New().String()[:13]
}

func generateETag() string {
	return "E" + uuid.New().String()[:32]
}

func defaultString(s, def string) string {
	if s == "" {
		return def
	}

	return s
}

func sortDistributionsByID(dists []*Distribution) {
	for i := range len(dists) {
		for j := i + 1; j < len(dists); j++ {
			if dists[i].ID > dists[j].ID {
				dists[i], dists[j] = dists[j], dists[i]
			}
		}
	}
}

func sortInvalidationsByID(invs []*Invalidation) {
	for i := range len(invs) {
		for j := i + 1; j < len(invs); j++ {
			if invs[i].ID > invs[j].ID {
				invs[i], invs[j] = invs[j], invs[i]
			}
		}
	}
}

func convertOriginsFromXML(origins *OriginsXML) *Origins {
	if origins == nil {
		return nil
	}

	result := &Origins{
		Quantity: origins.Quantity,
	}

	if origins.Items != nil {
		for _, o := range origins.Items.Origin {
			origin := Origin{
				ID:                    o.ID,
				DomainName:            o.DomainName,
				OriginPath:            o.OriginPath,
				ConnectionAttempts:    o.ConnectionAttempts,
				ConnectionTimeout:     o.ConnectionTimeout,
				OriginAccessControlID: o.OriginAccessControlID,
			}

			if o.S3OriginConfig != nil {
				origin.S3OriginConfig = &S3OriginConfig{
					OriginAccessIdentity: o.S3OriginConfig.OriginAccessIdentity,
				}
			}

			if o.CustomOriginConfig != nil {
				origin.CustomOriginConfig = &CustomOriginConfig{
					HTTPPort:               o.CustomOriginConfig.HTTPPort,
					HTTPSPort:              o.CustomOriginConfig.HTTPSPort,
					OriginProtocolPolicy:   o.CustomOriginConfig.OriginProtocolPolicy,
					OriginReadTimeout:      o.CustomOriginConfig.OriginReadTimeout,
					OriginKeepaliveTimeout: o.CustomOriginConfig.OriginKeepaliveTimeout,
				}
				if o.CustomOriginConfig.OriginSSLProtocols != nil {
					origin.CustomOriginConfig.OriginSSLProtocols = &OriginSSLProtocols{
						Quantity: o.CustomOriginConfig.OriginSSLProtocols.Quantity,
						Items:    o.CustomOriginConfig.OriginSSLProtocols.Items,
					}
				}
			}

			result.Items = append(result.Items, origin)
		}
	}

	return result
}

func convertDefaultCacheBehaviorFromXML(behavior *DefaultCacheBehaviorXML) *DefaultCacheBehavior {
	if behavior == nil {
		return nil
	}

	result := &DefaultCacheBehavior{
		TargetOriginID:       behavior.TargetOriginID,
		ViewerProtocolPolicy: behavior.ViewerProtocolPolicy,
		MinTTL:               behavior.MinTTL,
		DefaultTTL:           behavior.DefaultTTL,
		MaxTTL:               behavior.MaxTTL,
		Compress:             behavior.Compress,
		CachePolicyID:        behavior.CachePolicyID,
	}

	convertAllowedMethodsFromXML(behavior.AllowedMethods, result)
	convertForwardedValuesFromXML(behavior.ForwardedValues, result)
	convertTrustedSignersFromXML(behavior.TrustedSigners, result)
	convertTrustedKeyGroupsFromXML(behavior.TrustedKeyGroups, result)

	return result
}

func convertAllowedMethodsFromXML(methods *AllowedMethodsXML, result *DefaultCacheBehavior) {
	if methods == nil {
		return
	}

	result.AllowedMethods = &AllowedMethods{
		Quantity: methods.Quantity,
		Items:    methods.Items,
	}

	if methods.CachedMethods != nil {
		result.CachedMethods = &CachedMethods{
			Quantity: methods.CachedMethods.Quantity,
			Items:    methods.CachedMethods.Items,
		}
	}
}

func convertForwardedValuesFromXML(fv *ForwardedValuesXML, result *DefaultCacheBehavior) {
	if fv == nil {
		return
	}

	result.ForwardedValues = &ForwardedValues{
		QueryString: fv.QueryString,
	}

	if fv.Cookies != nil {
		result.ForwardedValues.Cookies = &CookiePreference{
			Forward: fv.Cookies.Forward,
		}
	}

	if fv.Headers != nil {
		result.ForwardedValues.Headers = &Headers{
			Quantity: fv.Headers.Quantity,
			Items:    fv.Headers.Items,
		}
	}
}

func convertTrustedSignersFromXML(ts *TrustedSignersXML, result *DefaultCacheBehavior) {
	if ts == nil {
		return
	}

	result.TrustedSigners = &TrustedSigners{
		Enabled:  ts.Enabled,
		Quantity: ts.Quantity,
		Items:    ts.Items,
	}
}

func convertTrustedKeyGroupsFromXML(tkg *TrustedKeyGroupsXML, result *DefaultCacheBehavior) {
	if tkg == nil {
		return
	}

	result.TrustedKeyGroups = &TrustedKeyGroups{
		Enabled:  tkg.Enabled,
		Quantity: tkg.Quantity,
		Items:    tkg.Items,
	}
}

func convertAliasesFromXML(aliases *AliasesXML) *Aliases {
	if aliases == nil {
		return nil
	}

	result := &Aliases{
		Quantity: aliases.Quantity,
	}

	if aliases.Items != nil {
		result.Items = aliases.Items.Items
	}

	return result
}

func convertViewerCertificateFromXML(cert *ViewerCertificateXML) *ViewerCertificate {
	if cert == nil {
		return &ViewerCertificate{
			CloudFrontDefaultCertificate: true,
			MinimumProtocolVersion:       "TLSv1",
		}
	}

	return &ViewerCertificate{
		CloudFrontDefaultCertificate: cert.CloudFrontDefaultCertificate,
		IAMCertificateID:             cert.IAMCertificateID,
		ACMCertificateArn:            cert.ACMCertificateArn,
		SSLSupportMethod:             cert.SSLSupportMethod,
		MinimumProtocolVersion:       cert.MinimumProtocolVersion,
	}
}

func convertPathsFromXML(paths *PathsXML) *Paths {
	if paths == nil {
		return nil
	}

	return &Paths{
		Quantity: paths.Quantity,
		Items:    paths.Items,
	}
}
