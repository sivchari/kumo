package s3

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	testQueueArn           = "arn:aws:sqs:us-east-1:000000000000:test-queue"
	wantInvalidArgumentErr = "InvalidArgument"
)

// filterRuleXML renders a single S3Key FilterRule element.
func filterRuleXML(name, value string) string {
	return fmt.Sprintf(`<FilterRule><Name>%s</Name><Value>%s</Value></FilterRule>`, name, value)
}

// keyFilterXML wraps FilterRule elements in a Filter/S3Key element. An
// empty rules argument yields no Filter element at all, matching a
// configuration with no key filter.
func keyFilterXML(rules ...string) string {
	if len(rules) == 0 {
		return ""
	}

	return `<Filter><S3Key>` + strings.Join(rules, "") + `</S3Key></Filter>`
}

func eventsXML(events []string) string {
	var b strings.Builder
	for _, e := range events {
		b.WriteString(`<Event>` + e + `</Event>`)
	}

	return b.String()
}

func queueConfigXML(id string, events []string, filter string) string {
	return `<QueueConfiguration><Id>` + id + `</Id><Queue>` + testQueueArn + `</Queue>` +
		eventsXML(events) + filter + `</QueueConfiguration>`
}

func topicConfigXML(id string, events []string, filter string) string {
	return `<TopicConfiguration><Id>` + id + `</Id><Topic>` + testTopicArn + `</Topic>` +
		eventsXML(events) + filter + `</TopicConfiguration>`
}

func lambdaConfigXML(id string, events []string, filter string) string {
	return `<CloudFunctionConfiguration><Id>` + id + `</Id><CloudFunction>` + testLambdaArn + `</CloudFunction>` +
		eventsXML(events) + filter + `</CloudFunctionConfiguration>`
}

// putNotificationConfigXMLResponse is like putNotificationConfigXML but
// returns the raw response instead of asserting success, so callers can
// exercise both accepted and rejected configurations.
func putNotificationConfigXMLResponse(svc *Service, bucket, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPut, "/"+bucket+"?notification", strings.NewReader(body))
	req.SetPathValue("bucket", bucket)

	w := httptest.NewRecorder()
	svc.PutBucketNotificationConfiguration(w, req)

	return w
}

type overlapTestCase struct {
	name    string
	configs []string
	wantErr bool
}

// overlapTestCases covers AWS's ambiguous filter rejection:
// https://docs.aws.amazon.com/AmazonS3/latest/userguide/notification-how-to-filtering.html
var overlapTestCases = []overlapTestCase{
	{
		name: "invalid-no-filter-topic-overlaps-prefix-filtered-topic",
		configs: []string{
			topicConfigXML("no-filter", []string{"s3:ReducedRedundancyLostObject"}, keyFilterXML()),
			topicConfigXML("prefix-filter", []string{"s3:ReducedRedundancyLostObject"},
				keyFilterXML(filterRuleXML("prefix", "images"))),
		},
		wantErr: true,
	},
	{
		name: "invalid-overlapping-suffixes-jpg-pg-with-intersecting-wildcard-events",
		configs: []string{
			topicConfigXML("suffix-jpg", []string{"s3:ObjectCreated:*"},
				keyFilterXML(filterRuleXML("suffix", "jpg"))),
			topicConfigXML("suffix-pg", []string{"s3:ObjectCreated:Put"},
				keyFilterXML(filterRuleXML("suffix", "pg"))),
		},
		wantErr: true,
	},
	{
		name: "invalid-prefix-images-suffix-jpg-overlaps-suffix-jpg-via-default-prefix",
		configs: []string{
			topicConfigXML("images-jpg", []string{"s3:ObjectCreated:Put"},
				keyFilterXML(filterRuleXML("prefix", "images"), filterRuleXML("suffix", "jpg"))),
			topicConfigXML("jpg-only", []string{"s3:ObjectCreated:Put"},
				keyFilterXML(filterRuleXML("suffix", "jpg"))),
		},
		wantErr: true,
	},
	{
		name: "valid-non-overlapping-prefixes-on-the-same-event",
		configs: []string{
			topicConfigXML("images", []string{"s3:ObjectCreated:Put"},
				keyFilterXML(filterRuleXML("prefix", "images/"))),
			topicConfigXML("logs", []string{"s3:ObjectCreated:Put"},
				keyFilterXML(filterRuleXML("prefix", "logs/"))),
		},
	},
	{
		name: "valid-non-overlapping-suffixes-on-the-same-event",
		configs: []string{
			topicConfigXML("jpg", []string{"s3:ObjectCreated:Put"},
				keyFilterXML(filterRuleXML("suffix", ".jpg"))),
			topicConfigXML("png", []string{"s3:ObjectCreated:Put"},
				keyFilterXML(filterRuleXML("suffix", ".png"))),
		},
	},
	{
		name: "valid-same-prefix-with-non-overlapping-suffixes",
		configs: []string{
			topicConfigXML("images-jpg", []string{"s3:ObjectCreated:Put"},
				keyFilterXML(filterRuleXML("prefix", "images"), filterRuleXML("suffix", ".jpg"))),
			topicConfigXML("images-png", []string{"s3:ObjectCreated:Put"},
				keyFilterXML(filterRuleXML("prefix", "images"), filterRuleXML("suffix", ".png"))),
		},
	},
	{
		name: "valid-identical-filter-on-non-intersecting-event-types",
		configs: []string{
			topicConfigXML("created", []string{"s3:ObjectCreated:Put"},
				keyFilterXML(filterRuleXML("prefix", "images/"))),
			topicConfigXML("removed", []string{"s3:ObjectRemoved:*"},
				keyFilterXML(filterRuleXML("prefix", "images/"))),
		},
	},
	{
		name: "invalid-two-prefix-rules-in-a-single-filter",
		configs: []string{
			topicConfigXML("dup-prefix", []string{"s3:ObjectCreated:Put"},
				keyFilterXML(filterRuleXML("prefix", "images/"), filterRuleXML("prefix", "photos/"))),
		},
		wantErr: true,
	},
	{
		name: "invalid-two-suffix-rules-in-a-single-filter",
		configs: []string{
			topicConfigXML("dup-suffix", []string{"s3:ObjectCreated:Put"},
				keyFilterXML(filterRuleXML("suffix", ".jpg"), filterRuleXML("suffix", ".png"))),
		},
		wantErr: true,
	},
	{
		name: "invalid-overlapping-filters-across-queue-and-topic",
		configs: []string{
			queueConfigXML("queue-cfg", []string{"s3:ObjectCreated:Put"}, keyFilterXML(filterRuleXML("prefix", "uploads/"))),
			topicConfigXML("topic-cfg", []string{"s3:ObjectCreated:Put"}, keyFilterXML(filterRuleXML("prefix", "uploads/photo"))),
		},
		wantErr: true,
	},
	{
		name: "invalid-overlapping-filters-across-lambda-and-topic",
		configs: []string{
			lambdaConfigXML("lambda-cfg", []string{"s3:ObjectCreated:Put"}, keyFilterXML(filterRuleXML("suffix", ".jpg"))),
			topicConfigXML("topic-cfg", []string{"s3:ObjectCreated:Put"}, keyFilterXML(filterRuleXML("suffix", ".jpg"))),
		},
		wantErr: true,
	},
	{
		name: "valid-non-overlapping-filters-across-queue-lambda-and-topic",
		configs: []string{
			queueConfigXML("queue-cfg", []string{"s3:ObjectCreated:Put"}, keyFilterXML(filterRuleXML("prefix", "uploads/"))),
			lambdaConfigXML("lambda-cfg", []string{"s3:ObjectCreated:Put"}, keyFilterXML(filterRuleXML("prefix", "logs/"))),
			topicConfigXML("topic-cfg", []string{"s3:ObjectCreated:Put"}, keyFilterXML(filterRuleXML("prefix", "archive/"))),
		},
	},
}

func TestPutBucketNotificationConfiguration_Overlap(t *testing.T) {
	t.Parallel()

	for i, tt := range overlapTestCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runOverlapCase(t, fmt.Sprintf("notif-overlap-%d", i), tt)
		})
	}
}

func runOverlapCase(t *testing.T, bucket string, tt overlapTestCase) {
	t.Helper()

	_, svc := newLambdaNotificationTestService(t, bucket)

	body := `<NotificationConfiguration>` + strings.Join(tt.configs, "") + `</NotificationConfiguration>`
	w := putNotificationConfigXMLResponse(svc, bucket, body)

	if tt.wantErr {
		assertInvalidArgument(t, w)

		return
	}

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}

	assertPersistedConfigCount(t, svc, bucket, len(tt.configs))
}

func assertInvalidArgument(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (body=%s)", w.Code, http.StatusBadRequest, w.Body.String())
	}

	var errResp ErrorResponse
	if err := xml.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}

	if errResp.Code != wantInvalidArgumentErr {
		t.Errorf("error code = %q, want %q", errResp.Code, wantInvalidArgumentErr)
	}
}

func assertPersistedConfigCount(t *testing.T, svc *Service, bucket string, want int) {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/"+bucket+"?notification", http.NoBody)
	req.SetPathValue("bucket", bucket)

	w := httptest.NewRecorder()
	svc.GetBucketNotificationConfiguration(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}

	var got NotificationConfiguration
	if err := xml.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal notification configuration: %v", err)
	}

	persisted := len(got.QueueConfigurations) + len(got.LambdaFunctionConfigurations) + len(got.TopicConfigurations)
	if persisted != want {
		t.Errorf("persisted configuration count = %d, want %d (body=%s)", persisted, want, w.Body.String())
	}
}
