package cloudwatch

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/sivchari/kumo/internal/server"
	"github.com/sivchari/kumo/internal/service"
)

// Compile-time check that Service implements io.Closer.
var _ io.Closer = (*Service)(nil)

func init() {
	var opts []Option
	if dir := os.Getenv("KUMO_DATA_DIR"); dir != "" {
		opts = append(opts, WithDataDir(dir))
	}

	storage := NewMemoryStorage("", opts...)
	service.Register(New(storage))
}

// Service implements the CloudWatch service.
type Service struct {
	storage Storage
}

// New creates a new CloudWatch service.
func New(storage Storage) *Service {
	return &Service{
		storage: storage,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return "monitoring"
}

// RegisterRoutes registers routes with the router.
// CloudWatch uses CBOR protocol, so routes are registered via DispatchCBORAction.
func (s *Service) RegisterRoutes(_ service.Router) {
	// CloudWatch uses RPC v2 CBOR protocol, routing is handled by DispatchCBORAction.
}

// ServiceName returns the Smithy service name for RPC v2 CBOR protocol.
func (s *Service) ServiceName() string {
	return "GraniteServiceVersion20100801"
}

// CBORProtocol is a marker method that indicates CloudWatch uses RPC v2 CBOR protocol.
func (s *Service) CBORProtocol() {}

// TargetPrefix returns the X-Amz-Target header prefix for CloudWatch.
//
// The form-encoded Query protocol path uses this so the unified dispatcher
// can wrap converted JSON requests with the right X-Amz-Target. Newer AWS
// SDKs go through the CBOR path above; terraform-provider-aws still uses
// the Query protocol against `/`.
func (s *Service) TargetPrefix() string {
	return "GraniteServiceVersion20100801"
}

// ServiceIdentifier returns the SDK service identifier sent in the User-Agent
// header by aws-sdk-go-v2.
//
// Note: this is "cloudwatch" (the SDK package name) not "monitoring" (the
// AWS service ID). terraform-provider-aws sends `api/cloudwatch#x.y.z`.
func (s *Service) ServiceIdentifier() string {
	return "cloudwatch"
}

// Actions returns the Query-protocol actions that DispatchAction handles.
func (s *Service) Actions() []string {
	return []string{
		"PutMetricData",
		"GetMetricData",
		"GetMetricStatistics",
		"ListMetrics",
		"PutMetricAlarm",
		"DeleteAlarms",
		"DescribeAlarms",
		"SetAlarmState",
		// Tag stubs — see tag_stubs.go.
		"ListTagsForResource",
		"TagResource",
		"UntagResource",
	}
}

// QueryProtocol is a marker method that indicates CloudWatch is reachable
// through the unified Query→JSON dispatcher in addition to CBOR.
func (s *Service) QueryProtocol() {}

// DispatchCBORAction handles RPC v2 CBOR protocol requests.
func (s *Service) DispatchCBORAction(w http.ResponseWriter, r *http.Request, operation string) {
	switch operation {
	case "PutMetricData":
		s.PutMetricDataCBOR(w, r)
	case "GetMetricData":
		s.GetMetricDataCBOR(w, r)
	case "GetMetricStatistics":
		s.GetMetricStatisticsCBOR(w, r)
	case "ListMetrics":
		s.ListMetricsCBOR(w, r)
	case "PutMetricAlarm":
		s.PutMetricAlarmCBOR(w, r)
	case "DeleteAlarms":
		s.DeleteAlarmsCBOR(w, r)
	case "DescribeAlarms":
		s.DescribeAlarmsCBOR(w, r)
	case "SetAlarmState":
		s.SetAlarmStateCBOR(w, r)
	default:
		server.WriteCBORError(w, "InvalidAction", "The action "+operation+" is not valid", http.StatusBadRequest)
	}
}

// Close saves the storage state if persistence is enabled.
func (s *Service) Close() error {
	if c, ok := s.storage.(io.Closer); ok {
		if err := c.Close(); err != nil {
			return fmt.Errorf("failed to close storage: %w", err)
		}
	}

	return nil
}
