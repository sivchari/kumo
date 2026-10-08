package cloudfront

import (
	"context"
	"encoding/xml"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// ensureResponseHeadersPolicyInit lazily initialises the response headers
// policy map, which a snapshot written before the feature existed does not
// contain.
func (s *MemoryStorage) ensureResponseHeadersPolicyInit() {
	if s.ResponseHeadersPolicies == nil {
		s.ResponseHeadersPolicies = make(map[string]*ResponseHeadersPolicy)
	}
}

// CreateResponseHeadersPolicy stores a new custom response headers policy.
// Names are unique per account (ResponseHeadersPolicyAlreadyExists).
func (s *MemoryStorage) CreateResponseHeadersPolicy(_ context.Context, cfg *ResponseHeadersPolicyConfig) (*ResponseHeadersPolicy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensureResponseHeadersPolicyInit()

	stored, err := normalizeResponseHeadersPolicyConfig(cfg)
	if err != nil {
		return nil, err
	}

	if err := s.responseHeadersPolicyNameAvailableLocked(stored.Name, ""); err != nil {
		return nil, err
	}

	policy := &ResponseHeadersPolicy{ID: uuid.NewString(), ETag: generateETag(), LastModifiedTime: time.Now().UTC(), Config: *stored}
	s.ResponseHeadersPolicies[policy.ID] = policy

	s.saveLocked()

	return policy.clone(), nil
}

// GetResponseHeadersPolicy returns a copy of the response headers policy.
func (s *MemoryStorage) GetResponseHeadersPolicy(_ context.Context, id string) (*ResponseHeadersPolicy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	policy, err := s.responseHeadersPolicyLocked(id)
	if err != nil {
		return nil, err
	}

	return policy.clone(), nil
}

// ListResponseHeadersPolicies returns the custom response headers policies
// sorted by id, starting after marker and limited to maxItems; nextMarker is
// the last id returned when more remain. kumo has no AWS-managed policies,
// so the managed filter yields nothing.
func (s *MemoryStorage) ListResponseHeadersPolicies(_ context.Context, policyType, marker string, maxItems int) ([]*ResponseHeadersPolicy, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if maxItems <= 0 {
		maxItems = responseHeadersPolicyListMaxItems
	}

	if policyType == responseHeadersPolicyTypeManaged {
		return nil, "", nil
	}

	all := make([]*ResponseHeadersPolicy, 0, len(s.ResponseHeadersPolicies))

	for _, policy := range s.ResponseHeadersPolicies {
		all = append(all, policy.clone())
	}

	page, nextMarker := pageByID(all, func(p *ResponseHeadersPolicy) string { return p.ID }, marker, maxItems)

	return page, nextMarker, nil
}

// UpdateResponseHeadersPolicy replaces the configuration when ifMatch is the
// current ETag and the new name is not taken by another policy.
func (s *MemoryStorage) UpdateResponseHeadersPolicy(_ context.Context, id string, cfg *ResponseHeadersPolicyConfig, ifMatch string) (*ResponseHeadersPolicy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	policy, err := s.responseHeadersPolicyForWriteLocked(id, ifMatch)
	if err != nil {
		return nil, err
	}

	stored, err := normalizeResponseHeadersPolicyConfig(cfg)
	if err != nil {
		return nil, err
	}

	if err := s.responseHeadersPolicyNameAvailableLocked(stored.Name, id); err != nil {
		return nil, err
	}

	policy.Config = *stored
	policy.ETag = generateETag()
	policy.LastModifiedTime = time.Now().UTC()

	s.saveLocked()

	return policy.clone(), nil
}

// DeleteResponseHeadersPolicy removes the policy when ifMatch is the current
// ETag and no cache behavior of any distribution, enabled or not, references
// it.
func (s *MemoryStorage) DeleteResponseHeadersPolicy(_ context.Context, id, ifMatch string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.responseHeadersPolicyForWriteLocked(id, ifMatch); err != nil {
		return err
	}

	if distID, ok := s.distributionReferencingLocked(func(b *DefaultCacheBehavior) bool { return b.ResponseHeadersPolicyID == id }); ok {
		return &Error{Code: errResponseHeadersPolicyInUse, Message: fmt.Sprintf("Cannot delete the response headers policy because it is attached to one or more cache behaviors of distribution %s.", distID)}
	}

	delete(s.ResponseHeadersPolicies, id)

	s.saveLocked()

	return nil
}

func (s *MemoryStorage) responseHeadersPolicyLocked(id string) (*ResponseHeadersPolicy, error) {
	policy, ok := s.ResponseHeadersPolicies[id]
	if !ok {
		return nil, &Error{Code: errNoSuchResponseHeadersPolicy, Message: "The response headers policy does not exist."}
	}

	return policy, nil
}

// responseHeadersPolicyForWriteLocked applies the If-Match protocol: the
// header is mandatory (InvalidIfMatchVersion) and must carry the current
// ETag (PreconditionFailed).
func (s *MemoryStorage) responseHeadersPolicyForWriteLocked(id, ifMatch string) (*ResponseHeadersPolicy, error) {
	if err := requireIfMatch(ifMatch); err != nil {
		return nil, err
	}

	policy, err := s.responseHeadersPolicyLocked(id)
	if err != nil {
		return nil, err
	}

	if err := checkIfMatch(policy.ETag, ifMatch); err != nil {
		return nil, err
	}

	return policy, nil
}

// responseHeadersPolicyNameAvailableLocked rejects a name already used by
// another response headers policy.
func (s *MemoryStorage) responseHeadersPolicyNameAvailableLocked(name, exceptID string) error {
	for _, other := range s.ResponseHeadersPolicies {
		if other.ID != exceptID && other.Config.Name == name {
			return &Error{Code: errResponseHeadersPolicyAlreadyExists, Message: "A response headers policy with this name already exists. You must provide a unique name."}
		}
	}

	return nil
}

// normalizeResponseHeadersPolicyConfig validates a request configuration and
// returns the copy to store.
func normalizeResponseHeadersPolicyConfig(cfg *ResponseHeadersPolicyConfig) (*ResponseHeadersPolicyConfig, error) {
	if err := validateResponseHeadersPolicyConfig(cfg); err != nil {
		return nil, err
	}

	out := cfg.clone()
	out.XMLName = xml.Name{}
	out.Xmlns = ""

	return out, nil
}

// validateResponseHeadersPolicyConfig enforces the required members, the
// documented value sets and limits, and Quantity / Items consistency.
func validateResponseHeadersPolicyConfig(cfg *ResponseHeadersPolicyConfig) error {
	switch {
	case cfg.Name == "":
		return invalidArgument("Name is required.")
	case utf8.RuneCountInString(cfg.Comment) > responseHeadersPolicyCommentMaxLength:
		return invalidArgument(fmt.Sprintf("The comment cannot be longer than %d characters.", responseHeadersPolicyCommentMaxLength))
	}

	for _, validate := range []func() error{
		func() error { return validateCorsConfig(cfg.CorsConfig) },
		func() error { return validateCustomHeadersConfig(cfg.CustomHeadersConfig) },
		func() error { return validateRemoveHeadersConfig(cfg.RemoveHeadersConfig) },
		func() error { return validateSecurityHeadersConfig(cfg.SecurityHeadersConfig) },
		func() error { return validateServerTimingHeadersConfig(cfg.ServerTimingHeadersConfig) },
	} {
		if err := validate(); err != nil {
			return err
		}
	}

	return nil
}

func validateCorsConfig(c *ResponseHeadersPolicyCorsConfig) error {
	switch {
	case c == nil:
		return nil
	case c.AccessControlAllowCredentials == nil:
		return invalidArgument("AccessControlAllowCredentials is required.")
	case c.OriginOverride == nil:
		return invalidArgument("OriginOverride is required.")
	case c.AccessControlAllowHeaders == nil:
		return invalidArgument("AccessControlAllowHeaders is required.")
	case c.AccessControlAllowMethods == nil:
		return invalidArgument("AccessControlAllowMethods is required.")
	case c.AccessControlAllowOrigins == nil:
		return invalidArgument("AccessControlAllowOrigins is required.")
	case c.AccessControlAllowHeaders.Items == nil, c.AccessControlAllowMethods.Items == nil, c.AccessControlAllowOrigins.Items == nil:
		return invalidArgument("Items is required in AccessControlAllowHeaders, AccessControlAllowMethods and AccessControlAllowOrigins.")
	}

	for _, method := range c.AccessControlAllowMethods.Items.Method {
		if !slices.Contains(responseHeadersPolicyAllowMethods, method) {
			return invalidArgument("AccessControlAllowMethods items must be one of GET, POST, OPTIONS, PUT, DELETE, PATCH, HEAD, ALL.")
		}
	}

	if err := checkQuantity(c.AccessControlAllowHeaders.Quantity, len(c.AccessControlAllowHeaders.Items.Header)); err != nil {
		return err
	}

	if err := checkQuantity(c.AccessControlAllowMethods.Quantity, len(c.AccessControlAllowMethods.Items.Method)); err != nil {
		return err
	}

	if err := checkQuantity(c.AccessControlAllowOrigins.Quantity, len(c.AccessControlAllowOrigins.Items.Origin)); err != nil {
		return err
	}

	if expose := c.AccessControlExposeHeaders; expose != nil {
		return checkQuantity(expose.Quantity, len(expose.Items.headers()))
	}

	return nil
}

func validateCustomHeadersConfig(c *ResponseHeadersPolicyCustomHeadersConfig) error {
	if c == nil {
		return nil
	}

	headers := c.Items.headers()

	for _, h := range headers {
		switch {
		case h.Header == nil:
			return invalidArgument("Header is required.")
		case h.Value == nil:
			return invalidArgument("Value is required.")
		case h.Override == nil:
			return invalidArgument("Override is required.")
		}
	}

	return checkQuantity(c.Quantity, len(headers))
}

func validateRemoveHeadersConfig(c *ResponseHeadersPolicyRemoveHeadersConfig) error {
	if c == nil {
		return nil
	}

	headers := c.Items.headers()

	for _, h := range headers {
		switch {
		case h.Header == nil:
			return invalidArgument("Header is required.")
		case !removableHeader(*h.Header):
			return invalidArgument(fmt.Sprintf("The header %s cannot be removed by a response headers policy.", *h.Header))
		}
	}

	return checkQuantity(c.Quantity, len(headers))
}

func removableHeader(name string) bool {
	if slices.ContainsFunc(responseHeadersPolicyUnremovableHeaders, func(h string) bool { return strings.EqualFold(h, name) }) {
		return false
	}

	return !slices.ContainsFunc(responseHeadersPolicyUnremovablePrefixes, func(p string) bool {
		return len(name) >= len(p) && strings.EqualFold(name[:len(p)], p)
	})
}

func validateSecurityHeadersConfig(c *ResponseHeadersPolicySecurityHeadersConfig) error {
	if c == nil {
		return nil
	}

	for _, validate := range []func() error{
		func() error { return validateContentSecurityPolicy(c.ContentSecurityPolicy) },
		func() error { return validateContentTypeOptions(c.ContentTypeOptions) },
		func() error { return validateFrameOptions(c.FrameOptions) },
		func() error { return validateReferrerPolicy(c.ReferrerPolicy) },
		func() error { return validateStrictTransportSecurity(c.StrictTransportSecurity) },
		func() error { return validateXSSProtection(c.XSSProtection) },
	} {
		if err := validate(); err != nil {
			return err
		}
	}

	return nil
}

func validateContentSecurityPolicy(v *ResponseHeadersPolicyContentSecurityPolicy) error {
	switch {
	case v == nil:
		return nil
	case v.Override == nil || v.ContentSecurityPolicy == nil:
		return invalidArgument("ContentSecurityPolicy requires Override and ContentSecurityPolicy.")
	case utf8.RuneCountInString(*v.ContentSecurityPolicy) > responseHeadersPolicyCSPMaxLength:
		return &Error{Code: errTooLongCSPInResponseHeadersPolicy, Message: fmt.Sprintf("The Content-Security-Policy header value cannot be longer than %d characters.", responseHeadersPolicyCSPMaxLength)}
	}

	return nil
}

func validateContentTypeOptions(v *ResponseHeadersPolicyContentTypeOptions) error {
	if v != nil && v.Override == nil {
		return invalidArgument("ContentTypeOptions requires Override.")
	}

	return nil
}

func validateFrameOptions(v *ResponseHeadersPolicyFrameOptions) error {
	if v != nil && (v.Override == nil || !slices.Contains(responseHeadersPolicyFrameOptions, v.FrameOption)) {
		return invalidArgument("FrameOptions requires Override and a FrameOption of DENY or SAMEORIGIN.")
	}

	return nil
}

func validateReferrerPolicy(v *ResponseHeadersPolicyReferrerPolicy) error {
	if v != nil && (v.Override == nil || !slices.Contains(responseHeadersPolicyReferrerPolicy, v.ReferrerPolicy)) {
		return invalidArgument("ReferrerPolicy requires Override and a valid ReferrerPolicy value.")
	}

	return nil
}

func validateStrictTransportSecurity(v *ResponseHeadersPolicyStrictTransportSecurity) error {
	if v != nil && (v.Override == nil || v.AccessControlMaxAgeSec == nil) {
		return invalidArgument("StrictTransportSecurity requires Override and AccessControlMaxAgeSec.")
	}

	return nil
}

func validateXSSProtection(v *ResponseHeadersPolicyXSSProtection) error {
	switch {
	case v == nil:
		return nil
	case v.Override == nil || v.Protection == nil:
		return invalidArgument("XSSProtection requires Override and Protection.")
	case v.ModeBlock != nil && *v.ModeBlock && v.ReportURI != nil:
		return invalidArgument("You cannot specify a ReportUri when ModeBlock is true.")
	}

	return nil
}

func validateServerTimingHeadersConfig(c *ResponseHeadersPolicyServerTimingHeadersConfig) error {
	switch {
	case c == nil:
		return nil
	case c.Enabled == nil:
		return invalidArgument("Enabled is required.")
	case c.SamplingRate == nil:
		return nil
	}

	rate := *c.SamplingRate
	if !(rate >= 0 && rate <= serverTimingSamplingRateMax) || math.Round(rate*serverTimingSamplingRateScale)/serverTimingSamplingRateScale != rate {
		return invalidArgument("SamplingRate must be a number 0-100 with up to four decimal places.")
	}

	return nil
}

// checkQuantity requires Quantity and its agreement with the number of items.
func checkQuantity(quantity *int, items int) error {
	switch {
	case quantity == nil:
		return invalidArgument("Quantity is required.")
	case *quantity != items:
		return &Error{Code: errInconsistentQuantities, Message: "The value of Quantity and the size of Items don't match."}
	}

	return nil
}

func (h *ResponseHeadersPolicyHeaders) headers() []string {
	if h == nil {
		return nil
	}

	return h.Header
}

func (l *ResponseHeadersPolicyCustomHeaderList) headers() []ResponseHeadersPolicyCustomHeader {
	if l == nil {
		return nil
	}

	return l.ResponseHeadersPolicyCustomHeader
}

func (l *ResponseHeadersPolicyRemoveHeaderList) headers() []ResponseHeadersPolicyRemoveHeader {
	if l == nil {
		return nil
	}

	return l.ResponseHeadersPolicyRemoveHeader
}

func (p *ResponseHeadersPolicy) clone() *ResponseHeadersPolicy {
	out := *p
	out.Config = *p.Config.clone()

	return &out
}

// clone deep-copies the configuration so neither callers nor later requests
// share pointers or slices with the stored policy.
func (c *ResponseHeadersPolicyConfig) clone() *ResponseHeadersPolicyConfig {
	out := *c
	out.CorsConfig = c.CorsConfig.clone()
	out.SecurityHeadersConfig = c.SecurityHeadersConfig.clone()

	if c.CustomHeadersConfig != nil {
		custom := ResponseHeadersPolicyCustomHeadersConfig{Quantity: clonePtr(c.CustomHeadersConfig.Quantity)}

		if items := c.CustomHeadersConfig.Items; items != nil {
			custom.Items = &ResponseHeadersPolicyCustomHeaderList{}

			for _, h := range items.ResponseHeadersPolicyCustomHeader {
				custom.Items.ResponseHeadersPolicyCustomHeader = append(custom.Items.ResponseHeadersPolicyCustomHeader, ResponseHeadersPolicyCustomHeader{Header: clonePtr(h.Header), Override: clonePtr(h.Override), Value: clonePtr(h.Value)})
			}
		}

		out.CustomHeadersConfig = &custom
	}

	if c.RemoveHeadersConfig != nil {
		remove := ResponseHeadersPolicyRemoveHeadersConfig{Quantity: clonePtr(c.RemoveHeadersConfig.Quantity)}

		if items := c.RemoveHeadersConfig.Items; items != nil {
			remove.Items = &ResponseHeadersPolicyRemoveHeaderList{}

			for _, h := range items.ResponseHeadersPolicyRemoveHeader {
				remove.Items.ResponseHeadersPolicyRemoveHeader = append(remove.Items.ResponseHeadersPolicyRemoveHeader, ResponseHeadersPolicyRemoveHeader{Header: clonePtr(h.Header)})
			}
		}

		out.RemoveHeadersConfig = &remove
	}

	if c.ServerTimingHeadersConfig != nil {
		out.ServerTimingHeadersConfig = &ResponseHeadersPolicyServerTimingHeadersConfig{
			Enabled:      clonePtr(c.ServerTimingHeadersConfig.Enabled),
			SamplingRate: clonePtr(c.ServerTimingHeadersConfig.SamplingRate),
		}
	}

	return &out
}

func (c *ResponseHeadersPolicyCorsConfig) clone() *ResponseHeadersPolicyCorsConfig {
	if c == nil {
		return nil
	}

	out := *c
	out.AccessControlAllowCredentials = clonePtr(c.AccessControlAllowCredentials)
	out.AccessControlAllowHeaders = c.AccessControlAllowHeaders.clone()
	out.AccessControlExposeHeaders = c.AccessControlExposeHeaders.clone()
	out.AccessControlMaxAgeSec = clonePtr(c.AccessControlMaxAgeSec)
	out.OriginOverride = clonePtr(c.OriginOverride)

	if m := c.AccessControlAllowMethods; m != nil {
		out.AccessControlAllowMethods = &ResponseHeadersPolicyMethodList{Quantity: clonePtr(m.Quantity)}
		if m.Items != nil {
			out.AccessControlAllowMethods.Items = &ResponseHeadersPolicyMethods{Method: slices.Clone(m.Items.Method)}
		}
	}

	if o := c.AccessControlAllowOrigins; o != nil {
		out.AccessControlAllowOrigins = &ResponseHeadersPolicyOriginList{Quantity: clonePtr(o.Quantity)}
		if o.Items != nil {
			out.AccessControlAllowOrigins.Items = &ResponseHeadersPolicyOrigins{Origin: slices.Clone(o.Items.Origin)}
		}
	}

	return &out
}

func (l *ResponseHeadersPolicyHeaderList) clone() *ResponseHeadersPolicyHeaderList {
	if l == nil {
		return nil
	}

	out := &ResponseHeadersPolicyHeaderList{Quantity: clonePtr(l.Quantity)}
	if l.Items != nil {
		out.Items = &ResponseHeadersPolicyHeaders{Header: slices.Clone(l.Items.Header)}
	}

	return out
}

func (c *ResponseHeadersPolicySecurityHeadersConfig) clone() *ResponseHeadersPolicySecurityHeadersConfig {
	if c == nil {
		return nil
	}

	out := ResponseHeadersPolicySecurityHeadersConfig{}

	if v := c.ContentSecurityPolicy; v != nil {
		out.ContentSecurityPolicy = &ResponseHeadersPolicyContentSecurityPolicy{ContentSecurityPolicy: clonePtr(v.ContentSecurityPolicy), Override: clonePtr(v.Override)}
	}

	if v := c.ContentTypeOptions; v != nil {
		out.ContentTypeOptions = &ResponseHeadersPolicyContentTypeOptions{Override: clonePtr(v.Override)}
	}

	if v := c.FrameOptions; v != nil {
		out.FrameOptions = &ResponseHeadersPolicyFrameOptions{FrameOption: v.FrameOption, Override: clonePtr(v.Override)}
	}

	if v := c.ReferrerPolicy; v != nil {
		out.ReferrerPolicy = &ResponseHeadersPolicyReferrerPolicy{Override: clonePtr(v.Override), ReferrerPolicy: v.ReferrerPolicy}
	}

	if v := c.StrictTransportSecurity; v != nil {
		out.StrictTransportSecurity = &ResponseHeadersPolicyStrictTransportSecurity{
			AccessControlMaxAgeSec: clonePtr(v.AccessControlMaxAgeSec),
			IncludeSubdomains:      clonePtr(v.IncludeSubdomains),
			Override:               clonePtr(v.Override),
			Preload:                clonePtr(v.Preload),
		}
	}

	if v := c.XSSProtection; v != nil {
		out.XSSProtection = &ResponseHeadersPolicyXSSProtection{ModeBlock: clonePtr(v.ModeBlock), Override: clonePtr(v.Override), Protection: clonePtr(v.Protection), ReportURI: clonePtr(v.ReportURI)}
	}

	return &out
}
