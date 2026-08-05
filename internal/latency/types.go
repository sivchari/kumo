// Package latency emulates AWS API latency from startup-loaded configuration.
package latency

import (
	"errors"
	"math"
	"time"
)

var (
	errLatencyRequired  = errors.New("latency rule requires fixedMs or percentile anchors")
	errMixedModes       = errors.New("latency rule cannot mix fixedMs with percentile anchors")
	errInvalidLatency   = errors.New("latency values must satisfy 0 <= p50Ms <= p95Ms <= p99Ms <= maxMs")
	errDuplicateRuleID  = errors.New("latency rule id must be unique")
	errRuleIDRequired   = errors.New("latency rule id is required")
	errRuleFileRequired = errors.New("latency config path is required")
)

// Config is the JSON document loaded at startup.
type Config struct {
	Seed  int64  `json:"seed,omitempty"`
	Rules []Rule `json:"rules"`
}

// Rule describes one latency emulation rule.
type Rule struct {
	ID      string  `json:"id"`
	Enabled bool    `json:"enabled"`
	Match   Match   `json:"match"`
	Latency Latency `json:"latency"`
}

// Match selects emulated AWS requests.
type Match struct {
	Service  string `json:"service,omitempty"`
	Action   string `json:"action,omitempty"`
	Method   string `json:"method,omitempty"`
	Path     string `json:"path,omitempty"`
	Pattern  string `json:"pattern,omitempty"`
	Resource string `json:"resource,omitempty"`
}

// Latency describes either a fixed delay or a percentile profile.
type Latency struct {
	FixedMs int `json:"fixedMs,omitempty"`
	P50Ms   int `json:"p50Ms,omitempty"`
	P95Ms   int `json:"p95Ms,omitempty"`
	P99Ms   int `json:"p99Ms,omitempty"`
	MaxMs   int `json:"maxMs,omitempty"`
}

// DurationAt returns the injected delay for quantile q in [0, 1].
func (l Latency) DurationAt(q float64) time.Duration {
	if l.FixedMs > 0 {
		return time.Duration(l.FixedMs) * time.Millisecond
	}

	switch {
	case q <= 0:
		return 0
	case q <= 0.50:
		return lerpDuration(0, l.P50Ms, q/0.50)
	case q <= 0.95:
		return lerpDuration(l.P50Ms, l.P95Ms, (q-0.50)/0.45)
	case q <= 0.99:
		return lerpDuration(l.P95Ms, l.P99Ms, (q-0.95)/0.04)
	case q >= 1:
		return time.Duration(l.MaxMs) * time.Millisecond
	default:
		return lerpDuration(l.P99Ms, l.MaxMs, (q-0.99)/0.01)
	}
}

// Validate checks that the latency config has exactly one valid mode.
func (l Latency) Validate() error {
	if l.hasNegativeValue() {
		return errInvalidLatency
	}

	hasFixed := l.FixedMs > 0
	hasProfile := l.P50Ms > 0 || l.P95Ms > 0 || l.P99Ms > 0 || l.MaxMs > 0

	if err := validateLatencyMode(hasFixed, hasProfile); err != nil {
		return err
	}

	if hasFixed {
		return nil
	}

	return l.validateProfile()
}

// Decision is the latency selected for one request.
type Decision struct {
	RuleID string
	Delay  time.Duration
}

func lerpDuration(fromMs, toMs int, t float64) time.Duration {
	ms := float64(fromMs) + (float64(toMs-fromMs) * t)

	return time.Duration(math.Round(ms)) * time.Millisecond
}

func (l Latency) hasNegativeValue() bool {
	return l.FixedMs < 0 || l.P50Ms < 0 || l.P95Ms < 0 || l.P99Ms < 0 || l.MaxMs < 0
}

func (l Latency) validateProfile() error {
	if l.P50Ms > l.P95Ms || l.P95Ms > l.P99Ms || l.P99Ms > l.MaxMs || l.MaxMs == 0 {
		return errInvalidLatency
	}

	return nil
}

func validateLatencyMode(hasFixed, hasProfile bool) error {
	switch {
	case hasFixed && hasProfile:
		return errMixedModes
	case !hasFixed && !hasProfile:
		return errLatencyRequired
	default:
		return nil
	}
}
