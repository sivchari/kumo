package s3

import "strings"

// notificationConfigError is a validation failure whose message is
// AWS's own user-facing wording (capitalized, punctuated), which is why it
// has a custom Error() method rather than going through errors.New —
// mirroring BucketError elsewhere in this package.
type notificationConfigError struct {
	message string
}

func (e *notificationConfigError) Error() string {
	return e.message
}

// errAmbiguousNotificationConfiguration is returned when two notification
// configurations (Queue, Lambda, or Topic, in any combination) have
// intersecting event types and overlapping key filters, matching the
// ambiguity AWS rejects. See
// https://docs.aws.amazon.com/AmazonS3/latest/userguide/notification-how-to-filtering.html
var errAmbiguousNotificationConfiguration = &notificationConfigError{
	message: "Configuration is ambiguously defined. Cannot have overlapping filter rules for the same event type.",
}

// errDuplicateFilterRule is returned when a single Filter declares more
// than one prefix rule or more than one suffix rule.
var errDuplicateFilterRule = &notificationConfigError{
	message: "Filter Rules must contain at most one Prefix rule and one Suffix rule.",
}

// notificationFilterRule is a normalized view of a single notification
// configuration's event types and key filter, used to detect overlaps
// across QueueConfigurations, LambdaFunctionConfigurations, and
// TopicConfigurations regardless of destination type.
type notificationFilterRule struct {
	events []string
	prefix string
	suffix string
}

// newNotificationFilterRule normalizes a configuration's events and Filter
// into a notificationFilterRule. A missing prefix or suffix rule defaults
// to "", which overlaps every other prefix or suffix per AWS's filtering
// semantics.
func newNotificationFilterRule(events []string, filter *NotificationFilter) (notificationFilterRule, error) {
	rule := notificationFilterRule{events: events}

	if filter == nil || filter.S3Key == nil {
		return rule, nil
	}

	var hasPrefix, hasSuffix bool

	for _, r := range filter.S3Key.FilterRules {
		switch {
		case strings.EqualFold(r.Name, "prefix"):
			if hasPrefix {
				return notificationFilterRule{}, errDuplicateFilterRule
			}

			hasPrefix = true
			rule.prefix = decodeFilterValue(r.Value)
		case strings.EqualFold(r.Name, "suffix"):
			if hasSuffix {
				return notificationFilterRule{}, errDuplicateFilterRule
			}

			hasSuffix = true
			rule.suffix = decodeFilterValue(r.Value)
		}
	}

	return rule, nil
}

// overlaps reports whether two configurations are ambiguous per AWS's
// rules: their event types intersect, their prefixes overlap, and their
// suffixes overlap.
func (r notificationFilterRule) overlaps(other notificationFilterRule) bool {
	if !eventTypesIntersect(r.events, other.events) {
		return false
	}

	return stringsOverlap(r.prefix, other.prefix, strings.HasPrefix) &&
		stringsOverlap(r.suffix, other.suffix, strings.HasSuffix)
}

// stringsOverlap reports whether a and b overlap under the given
// containment check (strings.HasPrefix or strings.HasSuffix): they
// overlap iff one contains the other, so some key could match both.
func stringsOverlap(a, b string, contains func(s, prefixOrSuffix string) bool) bool {
	return contains(a, b) || contains(b, a)
}

// eventTypesIntersect reports whether any event type in a matches any
// event type in b, honoring trailing wildcards (e.g. "s3:ObjectCreated:*"
// intersects "s3:ObjectCreated:Put").
func eventTypesIntersect(a, b []string) bool {
	for _, x := range a {
		for _, y := range b {
			if eventTypeIntersects(x, y) {
				return true
			}
		}
	}

	return false
}

func eventTypeIntersects(a, b string) bool {
	if a == b {
		return true
	}

	if strings.HasSuffix(a, "*") && strings.HasPrefix(b, strings.TrimSuffix(a, "*")) {
		return true
	}

	if strings.HasSuffix(b, "*") && strings.HasPrefix(a, strings.TrimSuffix(b, "*")) {
		return true
	}

	return false
}

// validateNotificationConfiguration rejects notification configurations
// AWS considers ambiguous: any two configurations across
// QueueConfigurations, LambdaFunctionConfigurations, and
// TopicConfigurations whose event types intersect and whose key filters
// overlap, and Filters that declare more than one prefix or suffix rule.
func validateNotificationConfiguration(config *NotificationConfiguration) error {
	var rules []notificationFilterRule

	for _, cfg := range config.QueueConfigurations {
		rule, err := newNotificationFilterRule(cfg.Events, cfg.Filter)
		if err != nil {
			return err
		}

		rules = append(rules, rule)
	}

	for _, cfg := range config.LambdaFunctionConfigurations {
		rule, err := newNotificationFilterRule(cfg.Events, cfg.Filter)
		if err != nil {
			return err
		}

		rules = append(rules, rule)
	}

	for _, cfg := range config.TopicConfigurations {
		rule, err := newNotificationFilterRule(cfg.Events, cfg.Filter)
		if err != nil {
			return err
		}

		rules = append(rules, rule)
	}

	for i := range rules {
		for j := i + 1; j < len(rules); j++ {
			if rules[i].overlaps(rules[j]) {
				return errAmbiguousNotificationConfiguration
			}
		}
	}

	return nil
}
