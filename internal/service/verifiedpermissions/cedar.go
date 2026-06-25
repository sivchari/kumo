package verifiedpermissions

import (
	"errors"
	"fmt"

	"github.com/cedar-policy/cedar-go"
	cedartypes "github.com/cedar-policy/cedar-go/types"
)

// errEmptyAttributeValue is returned when a context attribute has no set field.
var errEmptyAttributeValue = errors.New("empty or unsupported attribute value")

// validateStatement reports whether a Cedar policy statement parses.
func validateStatement(statement string) error {
	var policy cedar.Policy
	if err := policy.UnmarshalCedar([]byte(statement)); err != nil {
		return fmt.Errorf("invalid cedar policy: %w", err)
	}

	return nil
}

// buildPolicySet parses every stored static policy statement into a PolicySet,
// keyed by the policy id so determining policies can be reported back.
func buildPolicySet(policies map[string]*Policy) (*cedar.PolicySet, error) {
	ps := cedar.NewPolicySet()

	for id, p := range policies {
		var policy cedar.Policy
		if err := policy.UnmarshalCedar([]byte(p.Statement)); err != nil {
			return nil, fmt.Errorf("parse cedar policy %s: %w", id, err)
		}

		ps.Add(cedar.PolicyID(id), &policy)
	}

	return ps, nil
}

// buildRequest converts an AVP IsAuthorized input into a Cedar request.
func buildRequest(in *IsAuthorizedRequest) (cedar.Request, error) {
	var req cedar.Request

	if in.Principal != nil {
		req.Principal = cedar.NewEntityUID(
			cedartypes.EntityType(in.Principal.EntityType),
			cedartypes.String(in.Principal.EntityID),
		)
	}

	if in.Action != nil {
		req.Action = cedar.NewEntityUID(
			cedartypes.EntityType(in.Action.ActionType),
			cedartypes.String(in.Action.ActionID),
		)
	}

	if in.Resource != nil {
		req.Resource = cedar.NewEntityUID(
			cedartypes.EntityType(in.Resource.EntityType),
			cedartypes.String(in.Resource.EntityID),
		)
	}

	if in.Context != nil && in.Context.ContextMap != nil {
		record, err := contextToRecord(in.Context.ContextMap)
		if err != nil {
			return cedar.Request{}, err
		}

		req.Context = record
	}

	return req, nil
}

// contextToRecord maps an AVP context map to a Cedar record.
func contextToRecord(contextMap map[string]AttributeValue) (cedartypes.Record, error) {
	recordMap := cedartypes.RecordMap{}

	for key, attr := range contextMap {
		value, err := attrToValue(attr)
		if err != nil {
			return cedartypes.Record{}, fmt.Errorf("context key %q: %w", key, err)
		}

		recordMap[cedartypes.String(key)] = value
	}

	return cedartypes.NewRecord(recordMap), nil
}

// attrToValue maps a single AVP attribute value to a Cedar value, recursing
// into records and sets.
func attrToValue(attr AttributeValue) (cedartypes.Value, error) {
	switch {
	case attr.String != nil:
		return cedartypes.String(*attr.String), nil
	case attr.Long != nil:
		return cedartypes.Long(*attr.Long), nil
	case attr.Boolean != nil:
		return cedartypes.Boolean(*attr.Boolean), nil
	case attr.EntityIdentifier != nil:
		return cedar.NewEntityUID(
			cedartypes.EntityType(attr.EntityIdentifier.EntityType),
			cedartypes.String(attr.EntityIdentifier.EntityID),
		), nil
	case attr.Record != nil:
		record, err := contextToRecord(attr.Record)
		if err != nil {
			return nil, err
		}

		return record, nil
	case attr.Set != nil:
		values := make([]cedartypes.Value, 0, len(attr.Set))

		for _, item := range attr.Set {
			value, err := attrToValue(item)
			if err != nil {
				return nil, err
			}

			values = append(values, value)
		}

		return cedartypes.NewSet(values...), nil
	default:
		return nil, errEmptyAttributeValue
	}
}

// decide evaluates the request against the policy set and maps the Cedar
// decision and diagnostic onto the AVP response shape.
func decide(ps *cedar.PolicySet, req *cedar.Request) (string, []DeterminingPolicyItem, []EvaluationErrorItem) {
	var entities cedartypes.EntityMap

	decision, diag := cedar.Authorize(ps, entities, *req)

	result := "DENY"
	if decision == cedar.Allow {
		result = "ALLOW"
	}

	determining := make([]DeterminingPolicyItem, 0, len(diag.Reasons))
	for _, reason := range diag.Reasons {
		determining = append(determining, DeterminingPolicyItem{PolicyID: string(reason.PolicyID)})
	}

	errs := make([]EvaluationErrorItem, 0, len(diag.Errors))
	for _, evalErr := range diag.Errors {
		errs = append(errs, EvaluationErrorItem{ErrorDescription: evalErr.Message})
	}

	return result, determining, errs
}
