package chaos

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sivchari/kumo/internal/awsapi"
	"github.com/sivchari/kumo/internal/servicecatalog"
)

var (
	errInvalidLatencyProfile = errors.New("latency profile must satisfy 0 <= p50Ms <= p95Ms <= p99Ms <= maxMs")
	errInvalidRateLimit      = errors.New("rate_limit faults require limit.rps > 0 and limit.burst > 0")
)

// Engine evaluates chaos rules against normalized AWS requests.
type Engine struct {
	mu       sync.Mutex
	catalog  *servicecatalog.Catalog
	rules    map[string]Rule
	order    []string
	buckets  map[string]*tokenBucket
	rng      *rand.Rand
	clock    func() time.Time
	disabled bool
}

// NewEngine creates a chaos engine with no rules.
func NewEngine(catalog *servicecatalog.Catalog) *Engine {
	if catalog == nil {
		catalog = servicecatalog.NewDefault()
	}

	return &Engine{
		catalog: catalog,
		rules:   make(map[string]Rule),
		buckets: make(map[string]*tokenBucket),
		rng:     newRand(time.Now().UnixNano()),
		clock:   time.Now,
	}
}

// SetSeed makes probability and latency sampling deterministic.
func (e *Engine) SetSeed(seed int64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.rng = newRand(seed)
}

// SetClock replaces the engine clock. It is intended for tests.
func (e *Engine) SetClock(clock func() time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.clock = clock
}

// UpsertRule inserts or replaces a rule.
func (e *Engine) UpsertRule(rule *Rule) error {
	if rule.ID == "" {
		return fmt.Errorf("rule id is required")
	}

	rule.Match.Service = e.catalog.MustNormalize(rule.Match.Service)

	if err := validateFault(&rule.Fault); err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.rules[rule.ID]; !exists {
		e.order = append(e.order, rule.ID)
		sort.Strings(e.order)
	}

	e.rules[rule.ID] = *rule
	if rule.Fault.Type == FaultRateLimit && rule.Fault.Limit != nil {
		e.buckets[rule.ID] = newTokenBucket(rule.Fault.Limit.RPS, rule.Fault.Limit.Burst, e.clock())
	}

	return nil
}

// DeleteRule removes a rule by id.
func (e *Engine) DeleteRule(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	delete(e.rules, id)
	delete(e.buckets, id)

	for i, ruleID := range e.order {
		if ruleID == id {
			e.order = append(e.order[:i], e.order[i+1:]...)

			return
		}
	}
}

// Rules returns all rules sorted by id.
func (e *Engine) Rules() []Rule {
	e.mu.Lock()
	defer e.mu.Unlock()

	out := make([]Rule, 0, len(e.order))
	for _, id := range e.order {
		out = append(out, e.rules[id])
	}

	return out
}

// Reset removes all rules and limiter state.
func (e *Engine) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.rules = make(map[string]Rule)
	e.order = nil
	e.buckets = make(map[string]*tokenBucket)
}

// Evaluate returns the first effective fault decision for info.
func (e *Engine) Evaluate(info *awsapi.RequestInfo) *Decision {
	if info == nil {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.disabled {
		return nil
	}

	normalized := *info
	normalized.Service = e.catalog.MustNormalize(normalized.Service)
	now := e.clock()

	for _, id := range e.order {
		rule := e.rules[id]

		decision := e.evaluateRule(&rule, &normalized, now)
		if decision == nil {
			continue
		}

		return decision
	}

	return nil
}

func (e *Engine) evaluateRule(rule *Rule, info *awsapi.RequestInfo, now time.Time) *Decision {
	if !rule.Enabled || !matches(&rule.Match, info) {
		return nil
	}

	if rule.Fault.Until != nil && now.After(*rule.Fault.Until) {
		return nil
	}

	if !e.probabilityAllows(rule.Fault.Probability) {
		return nil
	}

	switch rule.Fault.Type {
	case FaultError:
		errSpec := defaultErrorSpec(&rule.Fault)

		return &Decision{RuleID: rule.ID, Error: &errSpec}
	case FaultDelay:
		return e.delayDecision(rule)
	case FaultRateLimit:
		return e.rateLimitDecision(rule, now)
	default:
		return nil
	}
}

func (e *Engine) probabilityAllows(probability float64) bool {
	return probability <= 0 || probability >= 1 || e.rng.Float64() <= probability
}

func (e *Engine) delayDecision(rule *Rule) *Decision {
	delay := time.Duration(rule.Fault.FixedMs) * time.Millisecond
	if rule.Fault.Latency != nil {
		delay = rule.Fault.Latency.DurationAt(e.rng.Float64())
	}

	if delay <= 0 {
		return nil
	}

	return &Decision{RuleID: rule.ID, Delay: delay}
}

func (e *Engine) rateLimitDecision(rule *Rule, now time.Time) *Decision {
	bucket := e.buckets[rule.ID]
	if bucket == nil && rule.Fault.Limit != nil {
		bucket = newTokenBucket(rule.Fault.Limit.RPS, rule.Fault.Limit.Burst, now)
		e.buckets[rule.ID] = bucket
	}

	if bucket == nil || bucket.allow(now) {
		return nil
	}

	errSpec := defaultRateLimitErrorSpec(rule.Fault.OnLimit)

	return &Decision{RuleID: rule.ID, Error: &errSpec}
}

func validateFault(fault *Fault) error {
	switch fault.Type {
	case FaultError:
		return nil
	case FaultDelay:
		if fault.Latency != nil {
			return fault.Latency.Validate()
		}

		return nil
	case FaultRateLimit:
		if fault.Limit == nil || fault.Limit.RPS <= 0 || fault.Limit.Burst <= 0 {
			return errInvalidRateLimit
		}

		return nil
	default:
		return fmt.Errorf("unsupported fault type %q", fault.Type)
	}
}

func matches(match *Match, info *awsapi.RequestInfo) bool {
	if match.Service != "" && !sameToken(match.Service, info.Service) {
		return false
	}

	if match.Action != "" && !sameToken(match.Action, info.Action) {
		return false
	}

	if match.Method != "" && !strings.EqualFold(match.Method, info.Method) {
		return false
	}

	if match.Path != "" && match.Path != info.Path {
		return false
	}

	if match.Pattern != "" && match.Pattern != info.Pattern {
		return false
	}

	return true
}

func sameToken(a, b string) bool {
	return compactToken(a) == compactToken(b)
}

func compactToken(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, ".", "")

	return s
}

func newRand(seed int64) *rand.Rand {
	//nolint:gosec // Chaos sampling must be reproducible when a seed is configured.
	return rand.New(rand.NewPCG(uint64(seed), uint64(seed>>32)))
}

type tokenBucket struct {
	rps    float64
	burst  float64
	tokens float64
	last   time.Time
}

func newTokenBucket(rps float64, burst int, now time.Time) *tokenBucket {
	return &tokenBucket{
		rps:    rps,
		burst:  float64(burst),
		tokens: float64(burst),
		last:   now,
	}
}

func (b *tokenBucket) allow(now time.Time) bool {
	elapsed := now.Sub(b.last).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * b.rps
		if b.tokens > b.burst {
			b.tokens = b.burst
		}

		b.last = now
	}

	if b.tokens < 1 {
		return false
	}

	b.tokens--

	return true
}
