package latency

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sivchari/kumo/internal/awsapi"
	"github.com/sivchari/kumo/internal/servicecatalog"
)

// Engine evaluates latency rules against normalized AWS requests.
type Engine struct {
	mu      sync.Mutex
	catalog *servicecatalog.Catalog
	rules   []Rule
	rng     *rand.Rand
}

// NewEngine creates a latency engine with no rules.
func NewEngine(catalog *servicecatalog.Catalog) *Engine {
	if catalog == nil {
		catalog = servicecatalog.NewDefault()
	}

	return &Engine{
		catalog: catalog,
		rng:     newRand(time.Now().UnixNano()),
	}
}

// LoadFile loads and validates a JSON latency config file.
func (e *Engine) LoadFile(path string) error {
	if path == "" {
		return errRuleFileRequired
	}

	//nolint:gosec // The latency emulator intentionally reads the caller-supplied config path.
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open latency config: %w", err)
	}

	defer func() {
		_ = file.Close()
	}()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()

	var config Config
	if err := decoder.Decode(&config); err != nil {
		return fmt.Errorf("decode latency config: %w", err)
	}

	if err := e.LoadConfig(config); err != nil {
		return fmt.Errorf("load latency config: %w", err)
	}

	return nil
}

// LoadConfig replaces the in-memory rules with validated config.
func (e *Engine) LoadConfig(config Config) error {
	rules := make([]Rule, 0, len(config.Rules))
	seen := make(map[string]struct{}, len(config.Rules))

	for i := range config.Rules {
		rule := config.Rules[i]

		if rule.ID == "" {
			return errRuleIDRequired
		}

		if _, ok := seen[rule.ID]; ok {
			return fmt.Errorf("%w: %s", errDuplicateRuleID, rule.ID)
		}

		seen[rule.ID] = struct{}{}
		rule.Match.Service = e.catalog.MustNormalize(rule.Match.Service)

		if err := rule.Latency.Validate(); err != nil {
			return fmt.Errorf("rule %q: %w", rule.ID, err)
		}

		rules = append(rules, rule)
	}

	rng := e.rng
	if config.Seed != 0 {
		rng = newRand(config.Seed)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.rules = rules
	e.rng = rng

	return nil
}

// Rules returns the configured rules in config-file order.
func (e *Engine) Rules() []Rule {
	e.mu.Lock()
	defer e.mu.Unlock()

	out := make([]Rule, len(e.rules))
	copy(out, e.rules)

	return out
}

// Evaluate returns the first matching latency decision for info.
func (e *Engine) Evaluate(info *awsapi.RequestInfo) *Decision {
	if info == nil {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	normalized := *info
	normalized.Service = e.catalog.MustNormalize(normalized.Service)

	for i := range e.rules {
		rule := &e.rules[i]

		if !rule.Enabled || !matches(&rule.Match, &normalized) {
			continue
		}

		delay := rule.Latency.DurationAt(e.rng.Float64())
		if delay <= 0 {
			continue
		}

		return &Decision{RuleID: rule.ID, Delay: delay}
	}

	return nil
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

	if match.Resource != "" && match.Resource != info.Resource {
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
	//nolint:gosec // Latency sampling must be reproducible when a seed is configured.
	return rand.New(rand.NewPCG(uint64(seed), uint64(seed>>32)))
}
