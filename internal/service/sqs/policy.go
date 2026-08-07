package sqs

import (
	"encoding/json"
	"fmt"
)

// policyVersion is the IAM policy language version AddPermission-generated
// documents use.
const policyVersion = "2012-10-17"

// policyActionPrefix is prepended to each action name AddPermission grants.
const policyActionPrefix = "SQS:"

// policyPrincipalARNFormat builds the root-account principal AddPermission
// grants access to, per AWS's documented AddPermission-generated policies.
const policyPrincipalARNFormat = "arn:aws:iam::%s:root"

// policyMaxActionsPerStatement is the documented limit on the number of
// actions a single AddPermission call may grant.
// https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_AddPermission.html
const policyMaxActionsPerStatement = 7

// policyDefaultIDSuffix names the Id AWS assigns to an AddPermission-generated
// policy that had no prior custom policy.
const policyDefaultIDSuffix = "/SQSDefaultPolicy"

// policyStatementSidKey is the JSON key identifying a statement's label
// within a policy document.
const policyStatementSidKey = "Sid"

// SQS action names, shared between the DispatchAction target-name dispatch
// table (handlers.go) and the AddPermission-grantable action set below. Both
// refer to the same AWS SQS API action names.
const (
	actionChangeMessageVisibility = "ChangeMessageVisibility"
	actionDeleteMessage           = "DeleteMessage"
	actionGetQueueAttributes      = "GetQueueAttributes"
	actionGetQueueURL             = "GetQueueUrl"
	actionPurgeQueue              = "PurgeQueue"
	actionReceiveMessage          = "ReceiveMessage"
	actionSendMessage             = "SendMessage"
)

// permissionActions are the actions eligible for cross-account sharing via
// AddPermission. Cross-account sharing does not apply to queue-management
// actions (AddPermission, CreateQueue, DeleteQueue, ListQueues,
// ListQueueTags, RemovePermission, SetQueueAttributes, TagQueue, UntagQueue).
// https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-customer-managed-policy-examples.html
var permissionActions = map[string]struct{}{
	"*":                           {},
	actionChangeMessageVisibility: {},
	actionDeleteMessage:           {},
	actionGetQueueAttributes:      {},
	actionGetQueueURL:             {},
	"ListDeadLetterSourceQueues":  {},
	actionPurgeQueue:              {},
	actionReceiveMessage:          {},
	actionSendMessage:             {},
}

// parsePolicy parses raw (a queue's stored Policy attribute) into a
// policyDocument, returning a fresh empty document if raw is unset.
func parsePolicy(raw, queueARN string) (*policyDocument, error) {
	if raw == "" {
		return &policyDocument{
			Version: policyVersion,
			ID:      queueARN + policyDefaultIDSuffix,
		}, nil
	}

	var doc policyDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return nil, fmt.Errorf("failed to parse queue policy: %w", err)
	}

	return &doc, nil
}

// marshal serializes d back to its string form. An empty statement list
// marshals to "" so GetQueueAttributes omits the Policy attribute, matching
// a queue that has never had a custom policy set.
func (d *policyDocument) marshal() (string, error) {
	if len(d.Statement) == 0 {
		return "", nil
	}

	data, err := json.Marshal(d)
	if err != nil {
		return "", fmt.Errorf("failed to marshal queue policy: %w", err)
	}

	return string(data), nil
}

// hasStatement reports whether a statement with the given Sid exists.
func (d *policyDocument) hasStatement(label string) bool {
	for _, stmt := range d.Statement {
		if sid, _ := stmt[policyStatementSidKey].(string); sid == label {
			return true
		}
	}

	return false
}

// removeStatement removes the statement with the given Sid, reporting
// whether one was found.
func (d *policyDocument) removeStatement(label string) bool {
	for i, stmt := range d.Statement {
		sid, _ := stmt[policyStatementSidKey].(string)
		if sid != label {
			continue
		}

		d.Statement = append(d.Statement[:i], d.Statement[i+1:]...)

		return true
	}

	return false
}

// buildPermissionStatement builds the policy statement AddPermission
// generates for label/awsAccountIDs/actions against queueARN.
func buildPermissionStatement(label, queueARN string, awsAccountIDs, actions []string) map[string]any {
	principals := make([]string, len(awsAccountIDs))
	for i, id := range awsAccountIDs {
		principals[i] = fmt.Sprintf(policyPrincipalARNFormat, id)
	}

	statementActions := make([]string, len(actions))
	for i, action := range actions {
		statementActions[i] = policyActionPrefix + action
	}

	return map[string]any{
		policyStatementSidKey: label,
		"Effect":              "Allow",
		"Principal":           map[string]any{"AWS": collapse(principals)},
		"Action":              collapse(statementActions),
		"Resource":            queueARN,
	}
}

// collapse returns values[0] when there is exactly one element, matching the
// policy AWS generates: single-member Principal/Action lists are stored as a
// plain string rather than a one-item array.
func collapse(values []string) any {
	if len(values) == 1 {
		return values[0]
	}

	return values
}
