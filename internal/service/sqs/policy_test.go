package sqs

import (
	"encoding/json"
	"testing"
)

func TestParsePolicy_EmptyGeneratesDefaultID(t *testing.T) {
	t.Parallel()

	doc, err := parsePolicy("", "arn:aws:sqs:us-east-1:000000000000:my-queue")
	if err != nil {
		t.Fatalf("parsePolicy() error = %v", err)
	}

	if doc.ID != "arn:aws:sqs:us-east-1:000000000000:my-queue/SQSDefaultPolicy" {
		t.Fatalf("doc.ID = %q, want default SQSDefaultPolicy ID", doc.ID)
	}

	if len(doc.Statement) != 0 {
		t.Fatalf("doc.Statement = %v, want empty", doc.Statement)
	}
}

func TestParsePolicy_PreservesUnknownFields(t *testing.T) {
	t.Parallel()

	raw := `{"Version":"2012-10-17","Statement":[{"Sid":"AllowSNS","Effect":"Allow","Principal":{"Service":"sns.amazonaws.com"},"Action":"sqs:SendMessage","Resource":"*","Condition":{"ArnEquals":{"aws:SourceArn":"arn:aws:sns:us-east-1:000000000000:my-topic"}}}]}`

	doc, err := parsePolicy(raw, "arn:aws:sqs:us-east-1:000000000000:my-queue")
	if err != nil {
		t.Fatalf("parsePolicy() error = %v", err)
	}

	if len(doc.Statement) != 1 {
		t.Fatalf("doc.Statement = %v, want 1 entry", doc.Statement)
	}

	if _, ok := doc.Statement[0]["Condition"]; !ok {
		t.Fatalf("doc.Statement[0] lost Condition field: %v", doc.Statement[0])
	}

	marshaled, err := doc.marshal()
	if err != nil {
		t.Fatalf("marshal() error = %v", err)
	}

	var roundTrip map[string]any
	if err := json.Unmarshal([]byte(marshaled), &roundTrip); err != nil {
		t.Fatalf("unmarshal roundtrip: %v", err)
	}

	stmts, _ := roundTrip["Statement"].([]any)
	if len(stmts) != 1 {
		t.Fatalf("round-tripped Statement = %v, want 1 entry", roundTrip["Statement"])
	}
}

func TestPolicyDocument_MarshalEmptyStatementsReturnsEmptyString(t *testing.T) {
	t.Parallel()

	doc, err := parsePolicy("", "arn:aws:sqs:us-east-1:000000000000:my-queue")
	if err != nil {
		t.Fatalf("parsePolicy() error = %v", err)
	}

	got, err := doc.marshal()
	if err != nil {
		t.Fatalf("marshal() error = %v", err)
	}

	if got != "" {
		t.Fatalf("marshal() = %q, want empty string for a policy with no statements", got)
	}
}

func TestPolicyDocument_HasAndRemoveStatement(t *testing.T) {
	t.Parallel()

	doc := &policyDocument{
		Version: policyVersion,
		Statement: []map[string]any{
			{policyStatementSidKey: "LabelA"},
			{policyStatementSidKey: "LabelB"},
		},
	}

	if !doc.hasStatement("LabelA") {
		t.Fatal("hasStatement(LabelA) = false, want true")
	}

	if doc.hasStatement("Missing") {
		t.Fatal("hasStatement(Missing) = true, want false")
	}

	if !doc.removeStatement("LabelA") {
		t.Fatal("removeStatement(LabelA) = false, want true")
	}

	if doc.hasStatement("LabelA") {
		t.Fatal("hasStatement(LabelA) after removal = true, want false")
	}

	if len(doc.Statement) != 1 {
		t.Fatalf("doc.Statement = %v, want 1 remaining entry", doc.Statement)
	}

	if doc.removeStatement("Missing") {
		t.Fatal("removeStatement(Missing) = true, want false")
	}
}

func TestBuildPermissionStatement_SingleValuesCollapseToStrings(t *testing.T) {
	t.Parallel()

	stmt := buildPermissionStatement("AliceSendMessage", "arn:aws:sqs:us-east-1:000000000000:my-queue", []string{"111111111111"}, []string{actionSendMessage})

	if stmt[policyStatementSidKey] != "AliceSendMessage" {
		t.Fatalf("Sid = %v, want AliceSendMessage", stmt[policyStatementSidKey])
	}

	if stmt["Effect"] != "Allow" {
		t.Fatalf("Effect = %v, want Allow", stmt["Effect"])
	}

	principal, ok := stmt["Principal"].(map[string]any)
	if !ok {
		t.Fatalf("Principal = %v, want map[string]any", stmt["Principal"])
	}

	if principal["AWS"] != "arn:aws:iam::111111111111:root" {
		t.Fatalf("Principal.AWS = %v, want single root ARN string", principal["AWS"])
	}

	if stmt["Action"] != "SQS:SendMessage" {
		t.Fatalf("Action = %v, want single SQS:SendMessage string", stmt["Action"])
	}

	if stmt["Resource"] != "arn:aws:sqs:us-east-1:000000000000:my-queue" {
		t.Fatalf("Resource = %v, want queue ARN", stmt["Resource"])
	}
}

func TestBuildPermissionStatement_MultipleValuesStayAsSlices(t *testing.T) {
	t.Parallel()

	stmt := buildPermissionStatement(
		"MultiAccount",
		"arn:aws:sqs:us-east-1:000000000000:my-queue",
		[]string{"111111111111", "222222222222"},
		[]string{actionSendMessage, actionReceiveMessage},
	)

	principal, ok := stmt["Principal"].(map[string]any)
	if !ok {
		t.Fatalf("Principal = %v, want map[string]any", stmt["Principal"])
	}

	awsPrincipals, ok := principal["AWS"].([]string)
	if !ok || len(awsPrincipals) != 2 {
		t.Fatalf("Principal.AWS = %v, want 2-element slice", principal["AWS"])
	}

	actions, ok := stmt["Action"].([]string)
	if !ok || len(actions) != 2 || actions[0] != "SQS:SendMessage" || actions[1] != "SQS:ReceiveMessage" {
		t.Fatalf("Action = %v, want [SQS:SendMessage SQS:ReceiveMessage]", stmt["Action"])
	}
}
