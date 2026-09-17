package s3

import (
	"encoding/base64"
	"errors"
	"net/http"
	"testing"
	"time"
)

const (
	testFieldACL     = "acl"
	testPolicyBucket = "demo-bucket"
	testPolicyNow    = "2026-01-01T00:00:00Z"
	testPolicyFuture = "2030-01-01T00:00:00Z"
)

// policyDoc wraps a conditions array in a not-yet-expired policy document.
func policyDoc(conditions string) string {
	return `{"expiration":"` + testPolicyFuture + `","conditions":` + conditions + `}`
}

func encodePolicy(doc string) string {
	return base64.StdEncoding.EncodeToString([]byte(doc))
}

func parseTestPolicy(t *testing.T, doc string) *postPolicy {
	t.Helper()

	policy, err := parsePostPolicy(encodePolicy(doc))
	if err != nil {
		t.Fatalf("parsePostPolicy(%s): unexpected error %v", doc, err)
	}

	return policy
}

func testPolicyNowTime(t *testing.T) time.Time {
	t.Helper()

	now, err := time.Parse(time.RFC3339, testPolicyNow)
	if err != nil {
		t.Fatalf("parse now: %v", err)
	}

	return now
}

// evaluatePolicy parses doc and evaluates it against the given form; key is the
// already-expanded object key and size the uploaded file size.
func evaluatePolicy(t *testing.T, doc, key string, form map[string][]string, size int64) error {
	t.Helper()

	return parseTestPolicy(t, doc).evaluate(testPolicyBucket, key, form, size, testPolicyNowTime(t))
}

// policyErrorOf asserts err is a *postPolicyError and returns it.
func policyErrorOf(t *testing.T, err error) *postPolicyError {
	t.Helper()

	var policyErr *postPolicyError
	if !errors.As(err, &policyErr) {
		t.Fatalf("expected *postPolicyError, got %T: %v", err, err)
	}

	return policyErr
}

func assertPolicyError(t *testing.T, err error, wantStatus int, wantCode, wantMessage string) {
	t.Helper()

	policyErr := policyErrorOf(t, err)
	if policyErr.Status != wantStatus || policyErr.Code != wantCode || policyErr.Message != wantMessage {
		t.Fatalf("got %d %s %q, want %d %s %q", policyErr.Status, policyErr.Code, policyErr.Message, wantStatus, wantCode, wantMessage)
	}
}

// assertSize checks an optional size element of a size error.
func assertSize(t *testing.T, name string, got *int64, want int64) {
	t.Helper()

	if got == nil {
		t.Fatalf("%s: expected %d, got nil", name, want)
	}

	if *got != want {
		t.Fatalf("%s: expected %d, got %d", name, want, *got)
	}
}

func assertConditionFailed(t *testing.T, err error, rendered string) {
	t.Helper()

	assertPolicyError(t, err, http.StatusForbidden, "AccessDenied", "Invalid according to Policy: Policy Condition failed: "+rendered)
}

func TestParsePostPolicy_RejectsMalformedDocuments(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		encoded string
		message string
	}{
		{"not base64", "%%%not-base64%%%", "The content of the form does not meet the conditions specified in the policy document."},
		{"not json", encodePolicy("{oops"), "Invalid Policy: Invalid JSON."},
		{"missing expiration", encodePolicy(`{"conditions":[]}`), "Invalid Policy: Missing expiration."},
		{"bad expiration", encodePolicy(`{"expiration":"tomorrow","conditions":[]}`), "Invalid Policy: Invalid expiration."},
		{"missing conditions", encodePolicy(`{"expiration":"` + testPolicyFuture + `"}`), "Invalid Policy: Missing conditions."},
		{"null conditions", encodePolicy(`{"expiration":"` + testPolicyFuture + `","conditions":null}`), "Invalid Policy: Missing conditions."},
		{"two properties", encodePolicy(policyDoc(`[{"acl":"private","key":"a"}]`)), "Invalid Policy: Invalid Simple-Condition: Simple-Conditions must have exactly one property specified."},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := parsePostPolicy(tc.encoded)
			assertPolicyError(t, err, http.StatusBadRequest, "InvalidPolicyDocument", tc.message)
		})
	}
}

func TestPostPolicy_ExactMatch(t *testing.T) {
	t.Parallel()

	form := map[string][]string{testFieldACL: {"public-read"}}

	if err := evaluatePolicy(t, policyDoc(`[{"acl":"public-read"}]`), "", form, 0); err != nil {
		t.Fatalf("object form: unexpected error %v", err)
	}

	if err := evaluatePolicy(t, policyDoc(`[["eq","$acl","public-read"]]`), "", form, 0); err != nil {
		t.Fatalf("array form: unexpected error %v", err)
	}

	err := evaluatePolicy(t, policyDoc(`[{"acl":"private"}]`), "", form, 0)
	assertConditionFailed(t, err, `["eq", "$acl", "private"]`)

	err = evaluatePolicy(t, policyDoc(`[["eq","$acl","private"]]`), "", form, 0)
	assertConditionFailed(t, err, `["eq", "$acl", "private"]`)

	err = evaluatePolicy(t, policyDoc(`[{"acl":"public-read"}]`), "", map[string][]string{}, 0)
	assertConditionFailed(t, err, `["eq", "$acl", "public-read"]`)
}

func TestPostPolicy_StartsWith(t *testing.T) {
	t.Parallel()

	form := map[string][]string{"success_action_redirect": {"http://localhost/done"}}

	if err := evaluatePolicy(t, policyDoc(`[["starts-with","$success_action_redirect","http://localhost"]]`), "", form, 0); err != nil {
		t.Fatalf("prefix match: unexpected error %v", err)
	}

	if err := evaluatePolicy(t, policyDoc(`[["starts-with","$success_action_redirect",""]]`), "", form, 0); err != nil {
		t.Fatalf("empty prefix: unexpected error %v", err)
	}

	if err := evaluatePolicy(t, policyDoc(`[["starts-with","$success_action_redirect",""]]`), "", map[string][]string{}, 0); err != nil {
		t.Fatalf("empty prefix with absent field: unexpected error %v", err)
	}

	err := evaluatePolicy(t, policyDoc(`[["starts-with","$success_action_redirect","https://"]]`), "", form, 0)
	assertConditionFailed(t, err, `["starts-with", "$success_action_redirect", "https://"]`)
}

func TestPostPolicy_StartsWithOnlyForSupportedFields(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		field string
		value string
	}{
		{"bucket", "bucket", testPolicyBucket},
		{"success_action_status", "success_action_status", "201"},
		{"x-amz-algorithm", "x-amz-algorithm", "AWS4-HMAC-SHA256"},
		{"x-amz-credential", "x-amz-credential", "AKID/20260101/us-east-1/s3/aws4_request"},
		{"x-amz-date", "x-amz-date", "20260101T000000Z"},
		{"x-amz-security-token", "x-amz-security-token", "token"},
		{"other x-amz header", "x-amz-storage-class", "STANDARD"},
		{"tagging", "tagging", "<Tagging/>"},
		{"toolkit field", "x-custom", "abc"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			form := map[string][]string{tc.field: {tc.value}}
			doc := policyDoc(`[["starts-with","$` + tc.field + `","` + tc.value[:1] + `"]]`)

			err := evaluatePolicy(t, doc, "", form, 0)
			assertConditionFailed(t, err, `["starts-with", "$`+tc.field+`", "`+tc.value[:1]+`"]`)
		})
	}

	form := map[string][]string{"x-amz-meta-owner": {"eric"}}
	if err := evaluatePolicy(t, policyDoc(`[["starts-with","$x-amz-meta-owner","er"]]`), "", form, 0); err != nil {
		t.Fatalf("x-amz-meta-* supports starts-with: unexpected error %v", err)
	}
}

func TestPostPolicy_ContentTypeListSemantics(t *testing.T) {
	t.Parallel()

	doc := policyDoc(`[["starts-with","$Content-Type","image/"]]`)

	pass := map[string][]string{"Content-Type": {"image/jpg,image/png,image/gif"}}
	if err := evaluatePolicy(t, doc, "", pass, 0); err != nil {
		t.Fatalf("all members match: unexpected error %v", err)
	}

	fail := map[string][]string{"Content-Type": {"image/jpg,text/plain"}}
	err := evaluatePolicy(t, doc, "", fail, 0)
	assertConditionFailed(t, err, `["starts-with", "$Content-Type", "image/"]`)

	// Only Content-Type is a list; commas elsewhere are ordinary characters.
	other := map[string][]string{"x-amz-meta-tags": {"a,b"}}
	if err := evaluatePolicy(t, policyDoc(`[["starts-with","$x-amz-meta-tags","a,"]]`), "", other, 0); err != nil {
		t.Fatalf("comma in other field: unexpected error %v", err)
	}
}

func TestPostPolicy_ContentLengthRange(t *testing.T) {
	t.Parallel()

	doc := policyDoc(`[["content-length-range",10,20]]`)
	form := map[string][]string{}

	for _, size := range []int64{10, 15, 20} {
		if err := evaluatePolicy(t, doc, "", form, size); err != nil {
			t.Fatalf("size %d within range: unexpected error %v", size, err)
		}
	}

	small := policyErrorOf(t, evaluatePolicy(t, doc, "", form, 5))
	assertPolicyError(t, small, http.StatusBadRequest, "EntityTooSmall", "Your proposed upload is smaller than the minimum allowed size")
	assertSize(t, "ProposedSize", small.ProposedSize, 5)
	assertSize(t, "MinSizeAllowed", small.MinSizeAllowed, 10)

	if small.MaxSizeAllowed != nil {
		t.Fatalf("EntityTooSmall must not report MaxSizeAllowed, got %d", *small.MaxSizeAllowed)
	}

	large := policyErrorOf(t, evaluatePolicy(t, doc, "", form, 30))
	assertPolicyError(t, large, http.StatusBadRequest, "EntityTooLarge", "Your proposed upload exceeds the maximum allowed size")
	assertSize(t, "ProposedSize", large.ProposedSize, 30)
	assertSize(t, "MaxSizeAllowed", large.MaxSizeAllowed, 20)
}

func TestPostPolicy_ContentLengthRangeReportsZeroValues(t *testing.T) {
	t.Parallel()

	form := map[string][]string{}

	empty := policyErrorOf(t, evaluatePolicy(t, policyDoc(`[["content-length-range",1,10]]`), "", form, 0))
	assertPolicyError(t, empty, http.StatusBadRequest, "EntityTooSmall", "Your proposed upload is smaller than the minimum allowed size")
	assertSize(t, "ProposedSize", empty.ProposedSize, 0)
	assertSize(t, "MinSizeAllowed", empty.MinSizeAllowed, 1)

	zeroMax := policyErrorOf(t, evaluatePolicy(t, policyDoc(`[["content-length-range",0,0]]`), "", form, 1))
	assertPolicyError(t, zeroMax, http.StatusBadRequest, "EntityTooLarge", "Your proposed upload exceeds the maximum allowed size")
	assertSize(t, "ProposedSize", zeroMax.ProposedSize, 1)
	assertSize(t, "MaxSizeAllowed", zeroMax.MaxSizeAllowed, 0)
}

func TestPostPolicy_UninterpretableConditionsFail(t *testing.T) {
	t.Parallel()

	form := map[string][]string{testFieldACL: {"private"}}

	cases := []struct {
		name      string
		condition string
		rendered  string
	}{
		{"range with strings", `["content-length-range","test","10"]`, `["content-length-range", "test", "10"]`},
		{"range with float", `["content-length-range",1.5,10]`, `["content-length-range", 1.5, 10]`},
		{"missing dollar", `["eq","acl","private"]`, `["eq", "acl", "private"]`},
		{"unknown operator", `["contains","$acl","priv"]`, `["contains", "$acl", "priv"]`},
		{"too few elements", `["eq","$acl"]`, `["eq", "$acl"]`},
		{"non-string value", `["eq","$acl",1]`, `["eq", "$acl", 1]`},
		{"non-string object value", `{"acl":1}`, `["eq", "$acl", 1]`},
		{"bare string", `"acl"`, `"acl"`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := evaluatePolicy(t, policyDoc(`[`+tc.condition+`]`), "", form, 0)
			assertConditionFailed(t, err, tc.rendered)
		})
	}
}

func TestPostPolicy_BucketAndKeyComeFromTheRequest(t *testing.T) {
	t.Parallel()

	if err := evaluatePolicy(t, policyDoc(`[{"bucket":"`+testPolicyBucket+`"}]`), "", map[string][]string{}, 0); err != nil {
		t.Fatalf("bucket match: unexpected error %v", err)
	}

	err := evaluatePolicy(t, policyDoc(`[{"bucket":"other"}]`), "", map[string][]string{}, 0)
	assertConditionFailed(t, err, `["eq", "$bucket", "other"]`)

	// The key is validated after ${filename} expansion, not as submitted.
	form := map[string][]string{postKeyField: {"user/eric/${filename}"}}
	if err := evaluatePolicy(t, policyDoc(`[["starts-with","$key","user/eric/"]]`), "user/eric/photo.jpg", form, 0); err != nil {
		t.Fatalf("expanded key: unexpected error %v", err)
	}

	err = evaluatePolicy(t, policyDoc(`[{"key":"user/eric/${filename}"}]`), "user/eric/photo.jpg", form, 0)
	assertConditionFailed(t, err, `["eq", "$key", "user/eric/${filename}"]`)
}

func TestPostPolicy_ExtraInputFields(t *testing.T) {
	t.Parallel()

	doc := policyDoc(`[["starts-with","$key",""]]`)

	err := evaluatePolicy(t, doc, "k", map[string][]string{postKeyField: {"k"}, "x-custom": {"1"}}, 0)
	assertPolicyError(t, err, http.StatusForbidden, "AccessDenied", "Invalid according to Policy: Extra input fields: x-custom")

	err = evaluatePolicy(t, doc, "k", map[string][]string{postKeyField: {"k"}, "zeta": {"1"}, "alpha": {"2"}}, 0)
	assertPolicyError(t, err, http.StatusForbidden, "AccessDenied", "Invalid according to Policy: Extra input fields: alpha, zeta")

	exempt := map[string][]string{
		postKeyField:       {"k"},
		"policy":           {"..."},
		"x-amz-signature":  {"sig"},
		"file":             {"ignored"},
		"x-ignore-tracker": {"1"},
	}
	if err := evaluatePolicy(t, doc, "k", exempt, 0); err != nil {
		t.Fatalf("exempt fields: unexpected error %v", err)
	}
}

func TestPostPolicy_FieldNamesAreCaseInsensitive(t *testing.T) {
	t.Parallel()

	form := map[string][]string{"content-type": {"text/csv"}, "X-Amz-Meta-Owner": {"eric"}}
	doc := policyDoc(`[["starts-with","$Content-Type",""],{"x-amz-meta-owner":"eric"}]`)

	if err := evaluatePolicy(t, doc, "", form, 0); err != nil {
		t.Fatalf("case-insensitive match: unexpected error %v", err)
	}

	// Names that differ only by case are resolved deterministically: the
	// lexicographically first submitted name wins.
	collision := map[string][]string{"X-Custom": {"A"}, "x-custom": {"B"}}
	if err := evaluatePolicy(t, policyDoc(`[{"x-custom":"A"}]`), "", collision, 0); err != nil {
		t.Fatalf("case collision: unexpected error %v", err)
	}
}

func TestPostPolicy_ConditionsAreEvaluatedInOrder(t *testing.T) {
	t.Parallel()

	doc := policyDoc(`[{"acl":"private"},{"acl":"public-read"}]`)

	err := evaluatePolicy(t, doc, "", map[string][]string{testFieldACL: {"public-read"}}, 0)
	assertConditionFailed(t, err, `["eq", "$acl", "private"]`)
}

func TestPostPolicy_Expiration(t *testing.T) {
	t.Parallel()

	expired := `{"expiration":"2020-01-01T00:00:00.000Z","conditions":[]}`

	err := evaluatePolicy(t, expired, "", map[string][]string{}, 0)
	assertPolicyError(t, err, http.StatusForbidden, "AccessDenied", "Invalid according to Policy: Policy expired.")

	if err := evaluatePolicy(t, policyDoc(`[]`), "", map[string][]string{}, 0); err != nil {
		t.Fatalf("future expiration: unexpected error %v", err)
	}
}
