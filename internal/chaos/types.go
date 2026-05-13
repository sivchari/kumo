// Package chaos injects AWS-aware failures and latency into emulated API requests.
package chaos

import (
	"math"
	"net/http"
	"time"
)

const (
	// FaultError returns an AWS-shaped error response.
	FaultError = "error"
	// FaultDelay injects latency before the real handler runs.
	FaultDelay = "delay"
	// FaultRateLimit applies a token-bucket limit and returns OnLimit when exceeded.
	FaultRateLimit = "rate_limit"
)

// Config is the JSON document accepted by future file-based chaos profiles.
type Config struct {
	Seed  int64  `json:"seed,omitempty"`
	Rules []Rule `json:"rules"`
}

// Rule describes one fault-injection rule.
type Rule struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
	Match   Match  `json:"match"`
	Fault   Fault  `json:"fault"`
}

// Match selects emulated AWS requests.
type Match struct {
	Service string `json:"service,omitempty"`
	Action  string `json:"action,omitempty"`
	Method  string `json:"method,omitempty"`
	Path    string `json:"path,omitempty"`
	Pattern string `json:"pattern,omitempty"`
}

// Fault describes the behavior to inject when a rule matches.
type Fault struct {
	Type        string          `json:"type"`
	Probability float64         `json:"probability,omitempty"`
	Status      int             `json:"status,omitempty"`
	Code        string          `json:"code,omitempty"`
	Message     string          `json:"message,omitempty"`
	FixedMs     int             `json:"fixedMs,omitempty"`
	Latency     *LatencyProfile `json:"latency,omitempty"`
	Limit       *RateLimit      `json:"limit,omitempty"`
	OnLimit     *FaultErrorSpec `json:"onLimit,omitempty"`
	Until       *time.Time      `json:"until,omitempty"`
}

// LatencyProfile describes a target injected-latency distribution.
type LatencyProfile struct {
	P50Ms int `json:"p50Ms,omitempty"`
	P95Ms int `json:"p95Ms,omitempty"`
	P99Ms int `json:"p99Ms,omitempty"`
	MaxMs int `json:"maxMs,omitempty"`
}

// DurationAt returns the injected delay for quantile q in [0, 1].
func (p LatencyProfile) DurationAt(q float64) time.Duration {
	switch {
	case q <= 0:
		return 0
	case q <= 0.50:
		return lerpDuration(0, p.P50Ms, q/0.50)
	case q <= 0.95:
		return lerpDuration(p.P50Ms, p.P95Ms, (q-0.50)/0.45)
	case q <= 0.99:
		return lerpDuration(p.P95Ms, p.P99Ms, (q-0.95)/0.04)
	case q >= 1:
		return time.Duration(p.MaxMs) * time.Millisecond
	default:
		return lerpDuration(p.P99Ms, p.MaxMs, (q-0.99)/0.01)
	}
}

// Validate checks that the profile anchors are monotonic.
func (p LatencyProfile) Validate() error {
	if p.P50Ms < 0 || p.P95Ms < 0 || p.P99Ms < 0 || p.MaxMs < 0 {
		return errInvalidLatencyProfile
	}

	if p.P50Ms > p.P95Ms || p.P95Ms > p.P99Ms || p.P99Ms > p.MaxMs {
		return errInvalidLatencyProfile
	}

	return nil
}

// RateLimit configures a token bucket.
type RateLimit struct {
	RPS   float64 `json:"rps"`
	Burst int     `json:"burst"`
}

// FaultErrorSpec is the AWS-shaped error returned by an injected fault.
type FaultErrorSpec struct {
	Status            int    `json:"status,omitempty"`
	Code              string `json:"code,omitempty"`
	Message           string `json:"message,omitempty"`
	RetryAfterSeconds int    `json:"retryAfterSeconds,omitempty"`
}

// Decision is an effective fault selected for one request.
type Decision struct {
	RuleID string
	Delay  time.Duration
	Error  *FaultErrorSpec
}

func defaultErrorSpec(f *Fault) FaultErrorSpec {
	status := f.Status
	if status == 0 {
		status = http.StatusServiceUnavailable
	}

	code := f.Code
	if code == "" {
		code = "ServiceUnavailable"
	}

	message := f.Message
	if message == "" {
		message = "Service is temporarily unavailable"
	}

	return FaultErrorSpec{
		Status:  status,
		Code:    code,
		Message: message,
	}
}

func defaultRateLimitErrorSpec(spec *FaultErrorSpec) FaultErrorSpec {
	if spec == nil {
		return FaultErrorSpec{
			Status:  http.StatusTooManyRequests,
			Code:    "ThrottlingException",
			Message: "Rate exceeded",
		}
	}

	out := *spec
	if out.Status == 0 {
		out.Status = http.StatusTooManyRequests
	}

	if out.Code == "" {
		out.Code = "ThrottlingException"
	}

	if out.Message == "" {
		out.Message = "Rate exceeded"
	}

	return out
}

func lerpDuration(fromMs, toMs int, t float64) time.Duration {
	ms := float64(fromMs) + (float64(toMs-fromMs) * t)

	return time.Duration(math.Round(ms)) * time.Millisecond
}
