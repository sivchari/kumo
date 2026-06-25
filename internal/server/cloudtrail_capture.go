package server

import (
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/cloudtrailevents"
)

const (
	defaultRegion      = "us-east-1"
	unknownEventSource = "unknown.amazonaws.com"
)

// managementEventCapturer records management API calls into the CloudTrail
// sink. The AWS region is resolved once at construction (from the environment,
// which does not change after startup) and reused for every captured event, so
// the request hot path never touches os.Getenv.
type managementEventCapturer struct {
	region string
}

// newManagementEventCapturer resolves the capture region once at startup.
func newManagementEventCapturer() *managementEventCapturer {
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		region = defaultRegion
	}

	return &managementEventCapturer{region: region}
}

// record records a management API call into the CloudTrail sink.
// It returns immediately (one atomic load) when no trail is logging, so the
// request hot path is essentially free in the common case.
//
// service/action are derived from the request: the X-Amz-Target header for the
// JSON protocol, or the Action form field for the Query protocol. REST data
// surfaces (e.g. S3 object operations) are intentionally not captured — those
// are CloudTrail data events, which are off by default.
func (c *managementEventCapturer) record(r *http.Request, isQuery bool) {
	if !cloudtrailevents.Global.Logging() {
		return
	}

	source, action := managementSourceAction(r, isQuery)
	if action == "" {
		return
	}

	cloudtrailevents.Global.Record(&cloudtrailevents.Event{
		EventTime:   time.Now().UTC(),
		EventSource: source,
		EventName:   action,
		AwsRegion:   c.region,
		SourceIP:    clientIP(r),
		UserAgent:   r.Header.Get("User-Agent"),
		RequestID:   uuid.New().String(),
	})
}

// managementSourceAction extracts (eventSource, eventName) from the request.
func managementSourceAction(r *http.Request, isQuery bool) (string, string) {
	if isQuery {
		if err := r.ParseForm(); err != nil {
			return "", ""
		}

		action := r.PostForm.Get("Action")

		return eventSourceFromUserAgent(r.Header.Get("User-Agent")), action
	}

	target := r.Header.Get("X-Amz-Target")

	idx := strings.LastIndex(target, ".")
	if idx < 0 {
		return "", ""
	}

	return eventSourceFromPrefix(target[:idx]), target[idx+1:]
}

// eventSourceFromPrefix turns a JSON target prefix (e.g. "DynamoDB_20120810")
// into a CloudTrail eventSource (e.g. "dynamodb.amazonaws.com").
func eventSourceFromPrefix(prefix string) string {
	token, _, _ := strings.Cut(prefix, "_")
	token = strings.TrimPrefix(token, "AWS")
	token = strings.TrimPrefix(token, "Amazon")

	if token == "" {
		return unknownEventSource
	}

	return strings.ToLower(token) + ".amazonaws.com"
}

// eventSourceFromUserAgent best-effort derives an eventSource from the SDK
// User-Agent (the Query dispatcher uses the same hint to route).
func eventSourceFromUserAgent(ua string) string {
	_, rest, found := strings.Cut(ua, "api/")
	if !found {
		return unknownEventSource
	}

	svc, _, _ := strings.Cut(rest, "#")
	svc = strings.TrimSpace(svc)

	if svc == "" {
		return unknownEventSource
	}

	return strings.ToLower(svc) + ".amazonaws.com"
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}
