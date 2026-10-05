package cloudfront

import (
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
)

const (
	xmlHeader       = `<?xml version="1.0" encoding="UTF-8"?>`
	cloudfrontXmlns = "http://cloudfront.amazonaws.com/doc/2020-05-31/"
)

// CreateDistribution handles the CreateDistribution operation, and delegates
// to CreateDistributionWithTags when the request carries the WithTags query.
//
// The router matches on method and path only, so both operations share this route.
func (s *Service) CreateDistribution(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Has("WithTags") {
		s.CreateDistributionWithTags(w, r)

		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeCloudFrontError(w, errMissingBody, "Request body is missing", http.StatusBadRequest)

		return
	}

	var req CreateDistributionRequest
	if err := xml.Unmarshal(body, &req); err != nil {
		writeCloudFrontError(w, errInvalidArgument, "Invalid request body", http.StatusBadRequest)

		return
	}

	s.createDistribution(w, r, &req, nil)
}

// createDistribution creates the distribution, stores tags (if any) under its
// ARN and writes the 201 response shared by both create operations.
func (s *Service) createDistribution(w http.ResponseWriter, r *http.Request, req *CreateDistributionRequest, tags map[string]string) {
	dist, err := s.storage.CreateDistribution(r.Context(), req)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	if len(tags) > 0 {
		if err := s.storage.TagResource(r.Context(), dist.ARN, tags); err != nil {
			handleStorageError(w, err)

			return
		}
	}

	resp := buildDistributionXML(dist)
	w.Header().Set("ETag", dist.ETag)
	w.Header().Set("Location", "/2020-05-31/distribution/"+dist.ID)
	writeXMLResponse(w, http.StatusCreated, resp)
}

// GetDistribution handles the GetDistribution operation.
func (s *Service) GetDistribution(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeCloudFrontError(w, errInvalidArgument, "Distribution ID is required", http.StatusBadRequest)

		return
	}

	dist, err := s.storage.GetDistribution(r.Context(), id)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	resp := buildDistributionXML(dist)
	w.Header().Set("ETag", dist.ETag)
	writeXMLResponse(w, http.StatusOK, resp)
}

// ListDistributions handles the ListDistributions operation.
func (s *Service) ListDistributions(w http.ResponseWriter, r *http.Request) {
	marker := r.URL.Query().Get("Marker")
	maxItemsStr := r.URL.Query().Get("MaxItems")
	maxItems := 100

	if maxItemsStr != "" {
		if v, err := strconv.Atoi(maxItemsStr); err == nil && v > 0 {
			maxItems = v
		}
	}

	dists, nextMarker, err := s.storage.ListDistributions(r.Context(), marker, maxItems)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	resp := buildDistributionListXML(dists, marker, maxItems, nextMarker)
	writeXMLResponse(w, http.StatusOK, resp)
}

// UpdateDistribution handles the UpdateDistribution operation.
func (s *Service) UpdateDistribution(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeCloudFrontError(w, errInvalidArgument, "Distribution ID is required", http.StatusBadRequest)

		return
	}

	etag := r.Header.Get("If-Match")
	if etag == "" {
		writeCloudFrontError(w, errPreconditionFailed, "The If-Match header is required", http.StatusPreconditionFailed)

		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeCloudFrontError(w, errMissingBody, "Request body is missing", http.StatusBadRequest)

		return
	}

	var req CreateDistributionRequest
	if err := xml.Unmarshal(body, &req); err != nil {
		writeCloudFrontError(w, errInvalidArgument, "Invalid request body", http.StatusBadRequest)

		return
	}

	dist, err := s.storage.UpdateDistribution(r.Context(), id, &req, etag)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	resp := buildDistributionXML(dist)
	w.Header().Set("ETag", dist.ETag)
	writeXMLResponse(w, http.StatusOK, resp)
}

// DeleteDistribution handles the DeleteDistribution operation.
func (s *Service) DeleteDistribution(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeCloudFrontError(w, errInvalidArgument, "Distribution ID is required", http.StatusBadRequest)

		return
	}

	etag := r.Header.Get("If-Match")
	if etag == "" {
		writeCloudFrontError(w, errPreconditionFailed, "The If-Match header is required", http.StatusPreconditionFailed)

		return
	}

	if err := s.storage.DeleteDistribution(r.Context(), id, etag); err != nil {
		handleStorageError(w, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CreateInvalidation handles the CreateInvalidation operation.
func (s *Service) CreateInvalidation(w http.ResponseWriter, r *http.Request) {
	distributionID := r.PathValue("id")
	if distributionID == "" {
		writeCloudFrontError(w, errInvalidArgument, "Distribution ID is required", http.StatusBadRequest)

		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeCloudFrontError(w, errMissingBody, "Request body is missing", http.StatusBadRequest)

		return
	}

	var req CreateInvalidationRequest
	if err := xml.Unmarshal(body, &req); err != nil {
		writeCloudFrontError(w, errInvalidArgument, "Invalid request body", http.StatusBadRequest)

		return
	}

	inv, err := s.storage.CreateInvalidation(r.Context(), distributionID, &req)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	resp := buildInvalidationXML(inv)
	w.Header().Set("Location", "/2020-05-31/distribution/"+distributionID+"/invalidation/"+inv.ID)
	writeXMLResponse(w, http.StatusCreated, resp)
}

// GetInvalidation handles the GetInvalidation operation.
func (s *Service) GetInvalidation(w http.ResponseWriter, r *http.Request) {
	distributionID := r.PathValue("id")
	invalidationID := r.PathValue("invalidationId")

	if distributionID == "" || invalidationID == "" {
		writeCloudFrontError(w, errInvalidArgument, "Distribution ID and Invalidation ID are required", http.StatusBadRequest)

		return
	}

	inv, err := s.storage.GetInvalidation(r.Context(), distributionID, invalidationID)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	resp := buildInvalidationXML(inv)
	writeXMLResponse(w, http.StatusOK, resp)
}

// GetDistributionConfig handles the GetDistributionConfig operation.
func (s *Service) GetDistributionConfig(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeCloudFrontError(w, errInvalidArgument, "Distribution ID is required", http.StatusBadRequest)

		return
	}

	dist, err := s.storage.GetDistribution(r.Context(), id)
	if err != nil {
		handleStorageError(w, err)

		return
	}

	resp := buildDistributionConfigXML(dist.DistributionConfig)
	w.Header().Set("ETag", dist.ETag)
	writeXMLResponse(w, http.StatusOK, resp)
}

// Helper functions.

func writeXMLResponse(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(status)
	_, _ = io.WriteString(w, xmlHeader)
	_ = xml.NewEncoder(w).Encode(v)
}

func writeCloudFrontError(w http.ResponseWriter, code, message string, status int) {
	resp := ErrorResponse{
		Xmlns: cloudfrontXmlns,
		Error: ErrorDetail{
			Type:    "Sender",
			Code:    code,
			Message: message,
		},
		RequestID: uuid.New().String(),
	}

	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("x-amzn-RequestId", resp.RequestID)
	w.WriteHeader(status)
	_, _ = io.WriteString(w, xmlHeader)
	_ = xml.NewEncoder(w).Encode(resp)
}

func handleStorageError(w http.ResponseWriter, err error) {
	var cfErr *Error
	if errors.As(err, &cfErr) {
		status := http.StatusBadRequest

		switch cfErr.Code {
		case errDistributionNotFound, errNoSuchInvalidation, errNoSuchOrigin, errNoSuchResource:
			status = http.StatusNotFound
		case errPreconditionFailed, errInvalidIfMatchVersion:
			status = http.StatusPreconditionFailed
		case errAccessDenied:
			status = http.StatusForbidden
		}

		writeCloudFrontError(w, cfErr.Code, cfErr.Message, status)

		return
	}

	writeCloudFrontError(w, "InternalError", "Internal server error", http.StatusInternalServerError)
}

func buildDistributionXML(dist *Distribution) *GetDistributionResult {
	return &GetDistributionResult{
		Xmlns:            cloudfrontXmlns,
		ID:               dist.ID,
		ARN:              dist.ARN,
		Status:           dist.Status,
		LastModifiedTime: dist.LastModifiedTime.Format(time.RFC3339),
		DomainName:       dist.DomainName,
		ActiveTrustedSigners: &ActiveTrustedSignersXML{
			Enabled:  dist.ActiveTrustedSigners.Enabled,
			Quantity: dist.ActiveTrustedSigners.Quantity,
		},
		ActiveTrustedKeyGroups: &ActiveTrustedKeyGroupsXML{
			Enabled:  dist.ActiveTrustedKeyGroups.Enabled,
			Quantity: dist.ActiveTrustedKeyGroups.Quantity,
		},
		DistributionConfig: buildDistributionConfigXML(dist.DistributionConfig),
	}
}

func buildDistributionConfigXML(config *DistributionConfig) *DistributionConfigXML {
	if config == nil {
		return nil
	}

	result := &DistributionConfigXML{
		CallerReference:   config.CallerReference,
		Comment:           config.Comment,
		Enabled:           config.Enabled,
		PriceClass:        config.PriceClass,
		DefaultRootObject: config.DefaultRootObject,
		HTTPVersion:       config.HTTPVersion,
		IsIPV6Enabled:     config.IsIPV6Enabled,
	}

	result.Origins = buildOriginsXML(config.Origins)
	// Always emitted, as CloudFront does; the Terraform AWS provider
	// dereferences OriginGroups.Quantity without a nil check.
	result.OriginGroups = &OriginGroupsXML{}
	result.DefaultCacheBehavior = buildDefaultCacheBehaviorXML(config.DefaultCacheBehavior)
	result.CacheBehaviors = buildCacheBehaviorsXML(config.CacheBehaviors)
	result.Aliases = buildAliasesConfigXML(config.Aliases)
	result.ViewerCertificate = buildViewerCertificateConfigXML(config.ViewerCertificate)

	return result
}

func buildOriginsXML(origins *Origins) *OriginsXML {
	if origins == nil {
		return nil
	}

	result := &OriginsXML{
		Quantity: origins.Quantity,
	}

	if len(origins.Items) == 0 {
		return result
	}

	result.Items = &OriginList{}

	for i := range origins.Items {
		origin := buildOriginXML(&origins.Items[i])
		result.Items.Origin = append(result.Items.Origin, origin)
	}

	return result
}

func buildOriginXML(o *Origin) OriginXML {
	origin := OriginXML{
		ID:                    o.ID,
		DomainName:            o.DomainName,
		OriginPath:            o.OriginPath,
		ConnectionAttempts:    o.ConnectionAttempts,
		ConnectionTimeout:     o.ConnectionTimeout,
		OriginAccessControlID: o.OriginAccessControlID,
	}

	if o.S3OriginConfig != nil {
		origin.S3OriginConfig = &S3OriginConfigXML{
			OriginAccessIdentity: o.S3OriginConfig.OriginAccessIdentity,
		}
	}

	if o.CustomOriginConfig != nil {
		origin.CustomOriginConfig = buildCustomOriginConfigXML(o.CustomOriginConfig)
	}

	return origin
}

func buildCustomOriginConfigXML(coc *CustomOriginConfig) *CustomOriginConfigXML {
	result := &CustomOriginConfigXML{
		HTTPPort:               coc.HTTPPort,
		HTTPSPort:              coc.HTTPSPort,
		OriginProtocolPolicy:   coc.OriginProtocolPolicy,
		OriginReadTimeout:      coc.OriginReadTimeout,
		OriginKeepaliveTimeout: coc.OriginKeepaliveTimeout,
	}

	if coc.OriginSSLProtocols != nil {
		result.OriginSSLProtocols = &OriginSSLProtocolsXML{
			Quantity: coc.OriginSSLProtocols.Quantity,
			Items:    coc.OriginSSLProtocols.Items,
		}
	}

	return result
}

func buildDefaultCacheBehaviorXML(dcb *DefaultCacheBehavior) *DefaultCacheBehaviorXML {
	if dcb == nil {
		return nil
	}

	// AllowedMethods (with CachedMethods), TrustedSigners, TrustedKeyGroups,
	// LambdaFunctionAssociations and FunctionAssociations are emitted even when
	// empty, as CloudFront does. The Terraform AWS provider dereferences them
	// without nil checks and panics when they are missing.
	result := &DefaultCacheBehaviorXML{
		TargetOriginID:          dcb.TargetOriginID,
		ViewerProtocolPolicy:    dcb.ViewerProtocolPolicy,
		MinTTL:                  dcb.MinTTL,
		DefaultTTL:              dcb.DefaultTTL,
		MaxTTL:                  dcb.MaxTTL,
		Compress:                dcb.Compress,
		SmoothStreaming:         dcb.SmoothStreaming,
		CachePolicyID:           dcb.CachePolicyID,
		OriginRequestPolicyID:   dcb.OriginRequestPolicyID,
		ResponseHeadersPolicyID: dcb.ResponseHeadersPolicyID,
		FieldLevelEncryptionID:  dcb.FieldLevelEncryptionID,
		RealtimeLogConfigArn:    dcb.RealtimeLogConfigArn,
	}

	result.AllowedMethods = buildAllowedMethodsXML(dcb)

	buildForwardedValuesXML(dcb.ForwardedValues, result)
	buildTrustedSignersXML(dcb.TrustedSigners, result)
	buildTrustedKeyGroupsXML(dcb.TrustedKeyGroups, result)

	result.LambdaFunctionAssociations = &LambdaFunctionAssociationsXML{}
	result.FunctionAssociations = &FunctionAssociationsXML{}

	return result
}

// buildAllowedMethodsXML returns the behavior's methods, defaulting to GET and
// HEAD for both allowed and cached methods as CloudFront does.
func buildAllowedMethodsXML(dcb *DefaultCacheBehavior) *AllowedMethodsXML {
	allowed, cached := defaultMethods(), defaultMethods()

	result := &AllowedMethodsXML{
		Quantity:      len(allowed),
		Items:         allowed,
		CachedMethods: &CachedMethodsXML{Quantity: len(cached), Items: cached},
	}

	if dcb.AllowedMethods != nil {
		result.Quantity = dcb.AllowedMethods.Quantity
		result.Items = dcb.AllowedMethods.Items
	}

	if dcb.CachedMethods != nil {
		result.CachedMethods = &CachedMethodsXML{
			Quantity: dcb.CachedMethods.Quantity,
			Items:    dcb.CachedMethods.Items,
		}
	}

	return result
}

func defaultMethods() []string {
	return []string{methodGet, methodHead}
}

func buildForwardedValuesXML(fv *ForwardedValues, result *DefaultCacheBehaviorXML) {
	if fv == nil {
		return
	}

	result.ForwardedValues = &ForwardedValuesXML{
		QueryString: fv.QueryString,
		Cookies:     buildCookiesXML(fv.Cookies),
	}

	if fv.Headers != nil {
		result.ForwardedValues.Headers = &HeadersXML{
			Quantity: fv.Headers.Quantity,
			Items:    fv.Headers.Items,
		}
	}

	if fv.QueryStringCacheKeys != nil {
		result.ForwardedValues.QueryStringCacheKeys = &QueryStringCacheKeysXML{
			Quantity: fv.QueryStringCacheKeys.Quantity,
			Items:    fv.QueryStringCacheKeys.Items,
		}
	}
}

func buildCookiesXML(cookies *CookiePreference) *CookiesXML {
	if cookies == nil {
		return nil
	}

	result := &CookiesXML{Forward: cookies.Forward}

	if cookies.WhitelistedNames != nil {
		result.WhitelistedNames = &CookieNamesXML{
			Quantity: cookies.WhitelistedNames.Quantity,
			Items:    cookies.WhitelistedNames.Items,
		}
	}

	return result
}

// buildCacheBehaviorsXML always emits the element, as CloudFront does, with
// Quantity 0 when the distribution has no ordered behaviors.
func buildCacheBehaviorsXML(behaviors *CacheBehaviors) *CacheBehaviorsXML {
	result := &CacheBehaviorsXML{}
	if behaviors == nil {
		return result
	}

	for i := range behaviors.Items {
		item := &behaviors.Items[i]
		result.Items = append(result.Items, CacheBehaviorXML{
			PathPattern:             item.PathPattern,
			DefaultCacheBehaviorXML: *buildDefaultCacheBehaviorXML(&item.DefaultCacheBehavior),
		})
	}

	result.Quantity = len(result.Items)

	return result
}

func buildTrustedSignersXML(ts *TrustedSigners, result *DefaultCacheBehaviorXML) {
	if ts == nil {
		result.TrustedSigners = &TrustedSignersXML{}

		return
	}

	result.TrustedSigners = &TrustedSignersXML{
		Enabled:  ts.Enabled,
		Quantity: ts.Quantity,
		Items:    ts.Items,
	}
}

func buildTrustedKeyGroupsXML(tkg *TrustedKeyGroups, result *DefaultCacheBehaviorXML) {
	if tkg == nil {
		result.TrustedKeyGroups = &TrustedKeyGroupsXML{}

		return
	}

	result.TrustedKeyGroups = &TrustedKeyGroupsXML{
		Enabled:  tkg.Enabled,
		Quantity: tkg.Quantity,
		Items:    tkg.Items,
	}
}

func buildAliasesConfigXML(aliases *Aliases) *AliasesXML {
	if aliases == nil {
		return nil
	}

	result := &AliasesXML{
		Quantity: aliases.Quantity,
	}

	if len(aliases.Items) > 0 {
		result.Items = &ItemsXML{
			Items: aliases.Items,
		}
	}

	return result
}

func buildViewerCertificateConfigXML(vc *ViewerCertificate) *ViewerCertificateXML {
	if vc == nil {
		return nil
	}

	return &ViewerCertificateXML{
		CloudFrontDefaultCertificate: vc.CloudFrontDefaultCertificate,
		IAMCertificateID:             vc.IAMCertificateID,
		ACMCertificateArn:            vc.ACMCertificateArn,
		SSLSupportMethod:             vc.SSLSupportMethod,
		MinimumProtocolVersion:       vc.MinimumProtocolVersion,
	}
}

func buildDistributionListXML(dists []*Distribution, marker string, maxItems int, nextMarker string) *DistributionListXML {
	result := &DistributionListXML{
		Xmlns:       cloudfrontXmlns,
		Marker:      marker,
		MaxItems:    maxItems,
		IsTruncated: nextMarker != "",
		Quantity:    len(dists),
	}

	if len(dists) > 0 {
		result.Items = &DistributionSummaryList{}

		for _, d := range dists {
			summary := buildDistributionSummaryXML(d)
			result.Items.DistributionSummary = append(result.Items.DistributionSummary, summary)
		}
	}

	if nextMarker != "" {
		result.NextMarker = nextMarker
	}

	return result
}

func buildDistributionSummaryXML(d *Distribution) DistributionSummaryXML {
	summary := DistributionSummaryXML{
		ID:                   d.ID,
		ARN:                  d.ARN,
		Status:               d.Status,
		LastModifiedTime:     d.LastModifiedTime.Format(time.RFC3339),
		DomainName:           d.DomainName,
		Enabled:              d.DistributionConfig.Enabled,
		Comment:              d.DistributionConfig.Comment,
		PriceClass:           d.DistributionConfig.PriceClass,
		HTTPVersion:          d.DistributionConfig.HTTPVersion,
		IsIPV6Enabled:        d.DistributionConfig.IsIPV6Enabled,
		CacheBehaviors:       buildCacheBehaviorsXML(d.DistributionConfig.CacheBehaviors),
		CustomErrorResponses: &CustomErrorResponsesXML{Quantity: 0},
		Restrictions: &RestrictionsXML{
			GeoRestriction: &GeoRestrictionXML{
				RestrictionType: "none",
				Quantity:        0,
			},
		},
		WebACLId: "",
		Staging:  false,
	}

	summary.Aliases = buildSummaryAliasesXML(d.DistributionConfig.Aliases)
	summary.Origins = buildSummaryOriginsXML(d.DistributionConfig.Origins)
	summary.DefaultCacheBehavior = buildDefaultCacheBehaviorXML(d.DistributionConfig.DefaultCacheBehavior)
	summary.ViewerCertificate = buildSummaryViewerCertificateXML(d.DistributionConfig.ViewerCertificate)

	return summary
}

func buildSummaryAliasesXML(aliases *Aliases) *AliasesXML {
	if aliases != nil {
		return &AliasesXML{
			Quantity: aliases.Quantity,
		}
	}

	return &AliasesXML{Quantity: 0}
}

func buildSummaryOriginsXML(origins *Origins) *OriginsXML {
	if origins == nil {
		return nil
	}

	result := &OriginsXML{
		Quantity: origins.Quantity,
	}

	if len(origins.Items) == 0 {
		return result
	}

	result.Items = &OriginList{}

	for _, o := range origins.Items {
		origin := OriginXML{
			ID:         o.ID,
			DomainName: o.DomainName,
			OriginPath: o.OriginPath,
		}

		if o.S3OriginConfig != nil {
			origin.S3OriginConfig = &S3OriginConfigXML{
				OriginAccessIdentity: o.S3OriginConfig.OriginAccessIdentity,
			}
		}

		result.Items.Origin = append(result.Items.Origin, origin)
	}

	return result
}

func buildSummaryViewerCertificateXML(vc *ViewerCertificate) *ViewerCertificateXML {
	if vc == nil {
		return nil
	}

	return &ViewerCertificateXML{
		CloudFrontDefaultCertificate: vc.CloudFrontDefaultCertificate,
		MinimumProtocolVersion:       vc.MinimumProtocolVersion,
	}
}

func buildInvalidationXML(inv *Invalidation) *InvalidationXML {
	result := &InvalidationXML{
		ID:         inv.ID,
		Status:     inv.Status,
		CreateTime: inv.CreateTime.Format(time.RFC3339),
	}

	if inv.InvalidationBatch != nil {
		result.InvalidationBatch = &InvalidationBatchXML{
			CallerReference: inv.InvalidationBatch.CallerReference,
		}
		if inv.InvalidationBatch.Paths != nil {
			result.InvalidationBatch.Paths = &PathsXML{
				Quantity: inv.InvalidationBatch.Paths.Quantity,
				Items:    inv.InvalidationBatch.Paths.Items,
			}
		}
	}

	return result
}
