package s3

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"time"
)

// POST policy vocabulary. See
// https://docs.aws.amazon.com/AmazonS3/latest/API/sigv4-HTTPPOSTConstructPolicy.html
const (
	postCondEq         = "eq"
	postCondStartsWith = "starts-with"
	postCondRange      = "content-length-range"

	postFieldBucket      = "bucket"
	postFieldContentType = "content-type"
	postFieldSignature   = "x-amz-signature"
	postIgnorePrefix     = "x-ignore-"

	errCodeAccessDenied   = "AccessDenied"
	errCodeEntityTooSmall = "EntityTooSmall"
	errCodeEntityTooLarge = "EntityTooLarge"

	msgPolicyExpired          = "Invalid according to Policy: Policy expired."
	msgPolicyConditionFailed  = "Invalid according to Policy: Policy Condition failed: "
	msgPolicyExtraInputFields = "Invalid according to Policy: Extra input fields: "
	msgPolicyUndecodable      = "The content of the form does not meet the conditions specified in the policy document."
	msgPolicyNotJSON          = "Invalid Policy: Invalid JSON."
	msgPolicyMissingExpiry    = "Invalid Policy: Missing expiration."
	msgPolicyInvalidExpiry    = "Invalid Policy: Invalid expiration."
	msgPolicyMissingConds     = "Invalid Policy: Missing conditions."
	msgPolicyMultiProperty    = "Invalid Policy: Invalid Simple-Condition: Simple-Conditions must have exactly one property specified."
	msgEntityTooSmall         = "Your proposed upload is smaller than the minimum allowed size"
	msgEntityTooLarge         = "Your proposed upload exceeds the maximum allowed size"
)

// postPolicyError is the S3 error a POST policy check produces. The size
// fields are only set for content-length-range violations.
type postPolicyError struct {
	Code    string
	Message string
	Status  int

	ProposedSize   *int64
	MinSizeAllowed *int64
	MaxSizeAllowed *int64
}

func (e *postPolicyError) Error() string {
	return e.Code + ": " + e.Message
}

func invalidPolicyDocument(message string) *postPolicyError {
	return &postPolicyError{Code: errCodeInvalidPolicyDocument, Message: message, Status: http.StatusBadRequest}
}

func policyAccessDenied(message string) *postPolicyError {
	return &postPolicyError{Code: errCodeAccessDenied, Message: message, Status: http.StatusForbidden}
}

// postCondition is one entry of a policy's conditions array. A condition kumo
// cannot interpret (wrong shape or types) is kept and fails evaluation, which
// is how S3 answers such conditions.
type postCondition struct {
	op       string
	field    string // lower-cased, without the leading "$"
	value    string
	minBytes int64
	maxBytes int64
	valid    bool
	rendered string // the condition as S3 echoes it in error messages
}

// postPolicy is a decoded POST policy document.
type postPolicy struct {
	expiration time.Time
	conditions []postCondition
}

// checkPostPolicy decodes the policy form field and evaluates it against the
// submitted form. kumo validates the document, not its signature.
func checkPostPolicy(encoded, bucket, key string, form map[string][]string, fileSize int64) error {
	policy, err := parsePostPolicy(encoded)
	if err != nil {
		return err
	}

	return policy.evaluate(bucket, key, form, fileSize, time.Now())
}

// parsePostPolicy decodes the base64 policy document. Structural problems are
// InvalidPolicyDocument errors; conditions with unexpected shapes are kept as
// unsatisfiable conditions (see postCondition).
func parsePostPolicy(encoded string) (*postPolicy, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, invalidPolicyDocument(msgPolicyUndecodable)
	}

	var doc struct {
		Expiration *string           `json:"expiration"`
		Conditions []json.RawMessage `json:"conditions"`
	}

	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, invalidPolicyDocument(msgPolicyNotJSON)
	}

	if doc.Expiration == nil {
		return nil, invalidPolicyDocument(msgPolicyMissingExpiry)
	}

	expiration, err := parsePolicyExpiration(*doc.Expiration)
	if err != nil {
		return nil, invalidPolicyDocument(msgPolicyInvalidExpiry)
	}

	if doc.Conditions == nil {
		return nil, invalidPolicyDocument(msgPolicyMissingConds)
	}

	conditions := make([]postCondition, 0, len(doc.Conditions))

	for _, rawCondition := range doc.Conditions {
		condition, err := parsePostCondition(rawCondition)
		if err != nil {
			return nil, err
		}

		conditions = append(conditions, condition)
	}

	return &postPolicy{expiration: expiration, conditions: conditions}, nil
}

func parsePostCondition(raw json.RawMessage) (postCondition, error) {
	trimmed := bytes.TrimSpace(raw)

	switch {
	case bytes.HasPrefix(trimmed, []byte("{")):
		return parseSimpleCondition(trimmed)
	case bytes.HasPrefix(trimmed, []byte("[")):
		return parseArrayCondition(trimmed), nil
	default:
		return postCondition{rendered: compactJSON(trimmed)}, nil
	}
}

// parseSimpleCondition handles the {"field": "value"} form, which S3 treats
// as ["eq", "$field", "value"].
func parseSimpleCondition(raw []byte) (postCondition, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || len(object) != 1 {
		return postCondition{}, invalidPolicyDocument(msgPolicyMultiProperty)
	}

	var (
		field string
		value json.RawMessage
	)

	for k, v := range object {
		field, value = k, v
	}

	opRaw, _ := json.Marshal(postCondEq)
	fieldRaw, _ := json.Marshal("$" + field)

	return conditionFromElements([]json.RawMessage{opRaw, fieldRaw, value}), nil
}

func parseArrayCondition(raw []byte) postCondition {
	var elements []json.RawMessage
	if err := json.Unmarshal(raw, &elements); err != nil {
		return postCondition{rendered: compactJSON(raw)}
	}

	return conditionFromElements(elements)
}

func conditionFromElements(elements []json.RawMessage) postCondition {
	condition := postCondition{rendered: renderCondition(elements)}
	if len(elements) != 3 {
		return condition
	}

	var op string
	if json.Unmarshal(elements[0], &op) != nil {
		return condition
	}

	switch op {
	case postCondEq, postCondStartsWith:
		condition.withMatch(op, elements[1], elements[2])
	case postCondRange:
		condition.withRange(elements[1], elements[2])
	}

	return condition
}

func (c *postCondition) withMatch(op string, fieldRaw, valueRaw json.RawMessage) {
	var field, value string
	if json.Unmarshal(fieldRaw, &field) != nil || json.Unmarshal(valueRaw, &value) != nil {
		return
	}

	if !strings.HasPrefix(field, "$") {
		return
	}

	c.op = op
	c.field = strings.ToLower(strings.TrimPrefix(field, "$"))
	c.value = value
	c.valid = true
}

func (c *postCondition) withRange(minRaw, maxRaw json.RawMessage) {
	minBytes, okMin := parseRangeBound(minRaw)
	maxBytes, okMax := parseRangeBound(maxRaw)

	if !okMin || !okMax {
		return
	}

	c.op = postCondRange
	c.minBytes = minBytes
	c.maxBytes = maxBytes
	c.valid = true
}

// parseRangeBound accepts JSON integers only; quoted numbers are not bounds.
func parseRangeBound(raw json.RawMessage) (int64, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] == '"' {
		return 0, false
	}

	var number json.Number
	if json.Unmarshal(trimmed, &number) != nil {
		return 0, false
	}

	bound, err := number.Int64()

	return bound, err == nil
}

// renderCondition formats a condition the way S3 does in error messages:
// a JSON array with ", " separators and the elements as submitted.
func renderCondition(elements []json.RawMessage) string {
	parts := make([]string, 0, len(elements))
	for _, element := range elements {
		parts = append(parts, compactJSON(element))
	}

	return "[" + strings.Join(parts, ", ") + "]"
}

func compactJSON(raw []byte) string {
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return string(bytes.TrimSpace(raw))
	}

	return buf.String()
}

// evaluate checks the policy against the request: expiration first, then
// every condition in policy order, then the rule that each form field must be
// covered by a condition. bucket comes from the request URL and key is the
// ${filename}-expanded object key.
func (p *postPolicy) evaluate(bucket, key string, form map[string][]string, fileSize int64, now time.Time) error {
	if now.After(p.expiration) {
		return policyAccessDenied(msgPolicyExpired)
	}

	fields := postFormFields(form, key)
	fields[postFieldBucket] = bucket

	covered := make(map[string]bool, len(p.conditions))

	for i := range p.conditions {
		condition := &p.conditions[i]
		if err := condition.check(fields, fileSize); err != nil {
			return err
		}

		covered[condition.field] = true
	}

	return checkExtraInputFields(form, covered)
}

// postFormFields lower-cases the form field names (S3 matches them
// case-insensitively) and keeps the first value of each. Names that differ
// only by case resolve to the lexicographically first submitted name.
func postFormFields(form map[string][]string, key string) map[string]string {
	names := make([]string, 0, len(form))
	for name := range form {
		names = append(names, name)
	}

	slices.Sort(names)

	fields := make(map[string]string, len(form)+1)

	for _, name := range names {
		lower := strings.ToLower(name)
		if _, seen := fields[lower]; seen || lower == postFileField {
			continue
		}

		fields[lower] = ""

		if values := form[name]; len(values) > 0 {
			fields[lower] = values[0]
		}
	}

	if key != "" {
		fields[postKeyField] = key
	}

	return fields
}

func (c *postCondition) check(fields map[string]string, fileSize int64) error {
	switch {
	case !c.valid:
		return c.failed()
	case c.op == postCondRange:
		return c.checkRange(fileSize)
	case c.op == postCondStartsWith:
		if !startsWithSupported(c.field) || !c.matchesPrefix(fields[c.field]) {
			return c.failed()
		}
	default:
		if fields[c.field] != c.value {
			return c.failed()
		}
	}

	return nil
}

func (c *postCondition) failed() error {
	return policyAccessDenied(msgPolicyConditionFailed + c.rendered)
}

func (c *postCondition) checkRange(fileSize int64) error {
	switch {
	case fileSize < c.minBytes:
		return &postPolicyError{
			Code: errCodeEntityTooSmall, Message: msgEntityTooSmall, Status: http.StatusBadRequest,
			ProposedSize: int64Ptr(fileSize), MinSizeAllowed: int64Ptr(c.minBytes),
		}
	case fileSize > c.maxBytes:
		return &postPolicyError{
			Code: errCodeEntityTooLarge, Message: msgEntityTooLarge, Status: http.StatusBadRequest,
			ProposedSize: int64Ptr(fileSize), MaxSizeAllowed: int64Ptr(c.maxBytes),
		}
	default:
		return nil
	}
}

func int64Ptr(v int64) *int64 {
	return &v
}

// matchesPrefix applies starts-with. A Content-Type value is a comma-separated
// list and every member must match; every other field is a plain string.
func (c *postCondition) matchesPrefix(value string) bool {
	if c.field != postFieldContentType {
		return strings.HasPrefix(value, c.value)
	}

	for _, member := range strings.Split(value, ",") {
		if !strings.HasPrefix(strings.TrimSpace(member), c.value) {
			return false
		}
	}

	return true
}

// startsWithSupported reports whether S3 allows starts-with for the field.
// Only the documented fields and user metadata support it; everything else
// (bucket, success_action_status, the signing fields, other x-amz-* headers,
// tagging, toolkit-specific fields) is exact-match only.
func startsWithSupported(field string) bool {
	switch field {
	case "acl", "cache-control", postFieldContentType, "content-disposition", "content-encoding", "expires",
		postKeyField, postRedirectField, postLegacyRedirect:
		return true
	}

	return strings.HasPrefix(field, postMetadataPrefix)
}

// checkExtraInputFields enforces that every submitted field (except file,
// policy, x-amz-signature and x-ignore-*) is covered by a condition.
func checkExtraInputFields(form map[string][]string, covered map[string]bool) error {
	var extra []string

	for name := range form {
		lower := strings.ToLower(name)
		if covered[lower] || isPolicyExemptField(lower) {
			continue
		}

		extra = append(extra, name)
	}

	if len(extra) == 0 {
		return nil
	}

	slices.Sort(extra)

	return policyAccessDenied(msgPolicyExtraInputFields + strings.Join(extra, ", "))
}

func isPolicyExemptField(lower string) bool {
	switch lower {
	case postPolicyField, postFieldSignature, postFileField:
		return true
	}

	return strings.HasPrefix(lower, postIgnorePrefix)
}
