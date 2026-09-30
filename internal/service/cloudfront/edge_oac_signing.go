package cloudfront

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"

	"github.com/sivchari/kumo/internal/vhost"
)

// The edge signs Lambda origin requests with a fixed local principal. kumo's
// function URL authentication is structural (no HMAC verification), so the
// key only has to be well-formed; it is what a function behind an origin
// access control sees in requestContext.authorizer.iam.accessKey.
const (
	edgeAccessKeyID      = "AKIAKUMOCLOUDFRONTOA"                  //nolint:gosec // G101: emulator principal, not a credential.
	edgeSecretAccessKey  = "kumo-cloudfront-origin-access-control" //nolint:gosec // G101: emulator principal, not a credential.
	defaultSigningRegion = "us-east-1"
	signingService       = "lambda"

	// emptyPayloadHash is the SHA-256 of an empty body, used for requests
	// without one.
	emptyPayloadHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	headerAuthorization = "Authorization"
	headerContentSHA256 = "X-Amz-Content-Sha256"
)

// errUnsignedPayload reports a body without the viewer-supplied payload hash;
// Lambda accepts no unsigned payloads, so CloudFront cannot sign such a request.
var errUnsignedPayload = errors.New("request body without x-amz-content-sha256")

// originSigning is the SigV4 signing the edge applies to origin requests of a
// Lambda function URL behind an origin access control of type lambda.
type originSigning struct {
	region string
	// alwaysOverride replaces any viewer Authorization (SigningBehavior always).
	alwaysOverride bool
	// forwardsAuthorization reports whether the matched behavior forwards the
	// viewer's Authorization header. With no-override a forwarded viewer
	// signature is passed through; an Authorization the behavior does not
	// forward is dropped and the edge signs instead.
	forwardsAuthorization bool
}

// shouldSign decides whether the edge signs, given the viewer's Authorization header.
func (o *originSigning) shouldSign(viewerAuthorization string) bool {
	if o.alwaysOverride {
		return true
	}

	return viewerAuthorization == "" || !o.forwardsAuthorization
}

// requiresPayloadHash reports whether the viewer must have sent
// x-amz-content-sha256: the edge is about to sign a request with a body.
func (o *originSigning) requiresPayloadHash(r *http.Request) bool {
	return requestHasBody(r) && o.shouldSign(r.Header.Get(headerAuthorization)) && r.Header.Get(headerContentSHA256) == ""
}

// sign replaces the viewer's Authorization with the edge's SigV4 signature
// over the request as it reaches the origin (Host already set, viewer headers
// copied), using the viewer-supplied payload hash when there is a body.
func (o *originSigning) sign(req, viewer *http.Request, hasBody bool) error {
	if !o.shouldSign(viewer.Header.Get(headerAuthorization)) {
		return nil
	}

	payloadHash := emptyPayloadHash

	if hasBody {
		payloadHash = viewer.Header.Get(headerContentSHA256)
		if payloadHash == "" {
			return errUnsignedPayload
		}
	}

	req.Header.Del(headerAuthorization)
	req.Header.Set(headerContentSHA256, payloadHash)

	creds := aws.Credentials{AccessKeyID: edgeAccessKeyID, SecretAccessKey: edgeSecretAccessKey}

	if err := v4.NewSigner().SignHTTP(req.Context(), creds, req, payloadHash, signingService, o.region, time.Now()); err != nil {
		return fmt.Errorf("sign origin request: %w", err)
	}

	return nil
}

// originSigning resolves what the matched origin needs: nil when it has no
// origin access control, when the control is not of type lambda (kumo's S3
// verifies no signatures, so signing s3 origins would change nothing) or
// when its SigningBehavior is never.
func (s *Service) originSigning(ctx context.Context, o *Origin, behavior *DefaultCacheBehavior) *originSigning {
	if o.OriginAccessControlID == "" {
		return nil
	}

	oac, err := s.storage.GetOriginAccessControl(ctx, o.OriginAccessControlID)
	if err != nil || oac.Config.OriginAccessControlOriginType != oacOriginTypeLambda {
		return nil
	}

	switch oac.Config.SigningBehavior {
	case oacSigningAlways:
		return &originSigning{region: lambdaURLRegion(o.DomainName), alwaysOverride: true}
	case oacSigningNoOverride:
		return &originSigning{region: lambdaURLRegion(o.DomainName), forwardsAuthorization: behaviorForwardsAuthorization(behavior)}
	}

	return nil
}

// behaviorForwardsAuthorization reports whether the behavior's legacy header
// whitelist forwards Authorization (or every header). Cache policy ids are
// opaque to kumo, so this is the only forwarding rule it can evaluate.
func behaviorForwardsAuthorization(behavior *DefaultCacheBehavior) bool {
	if behavior == nil || behavior.ForwardedValues == nil || behavior.ForwardedValues.Headers == nil {
		return false
	}

	return slices.ContainsFunc(behavior.ForwardedValues.Headers.Items, func(name string) bool {
		return name == "*" || strings.EqualFold(name, headerAuthorization)
	})
}

// lambdaURLRegion takes the signing region from <id>.lambda-url.<region>.on.aws;
// the kumo-local .localhost form carries none and follows AWS_DEFAULT_REGION.
func lambdaURLRegion(domain string) string {
	host := strings.ToLower(vhost.StripPort(domain))

	if _, rest, found := strings.Cut(host, ".lambda-url."); found {
		if region, suffix, ok := strings.Cut(rest, "."); ok && region != "" && suffix == "on.aws" {
			return region
		}
	}

	if region := os.Getenv("AWS_DEFAULT_REGION"); region != "" {
		return region
	}

	return defaultSigningRegion
}

func requestHasBody(r *http.Request) bool {
	return r.Body != nil && r.Body != http.NoBody && r.ContentLength != 0
}

// writeForbidden answers the way a function URL refuses a request.
func writeForbidden(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`{"Message":"Forbidden"}`))
}
