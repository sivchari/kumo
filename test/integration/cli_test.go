//go:build integration

// End-to-end goldens for the generated kumo CLI (cli/gen_*.go, see
// cmd/cli-gen). These drive the actual kumo binary as a subprocess and
// exercise a representative create + data-plane read/write per migrated
// service, in addition to the direct SDK coverage in the other _test.go
// files in this package.
package integration

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/sivchari/golden"
)

func TestCLI_SQS_QueueLifecycle(t *testing.T) {
	queueName := "test-cli-sqs-queue"

	createOut := runCLI(t, "sqs", "create-queue", "--queue-name", queueName)
	created := decodeCLIJSON(t, createOut)
	golden.New(t, golden.WithIgnoreFields("QueueUrl", "ResultMetadata")).Assert(t.Name()+"_create", createOut)

	queueURL, _ := created["QueueUrl"].(string)
	if queueURL == "" {
		t.Fatalf("create-queue: no QueueUrl in output: %s", createOut)
	}

	client := newSQSClient(t)
	t.Cleanup(func() {
		_, _ = client.DeleteQueue(context.Background(), &sqs.DeleteQueueInput{
			QueueUrl: aws.String(queueURL),
		})
	})

	sendOut := runCLI(t, "sqs", "send-message", "--queue-url", queueURL, "--message-body", "Hello, CLI SQS!")
	golden.New(t, golden.WithIgnoreFields(
		"MessageId", "MD5OfMessageBody", "SequenceNumber", "ResultMetadata",
	)).Assert(t.Name()+"_send", sendOut)

	recvOut := runCLI(t, "sqs", "receive-message", "--queue-url", queueURL, "--max-number-of-messages", "1")
	golden.New(t, golden.WithIgnoreFields(
		"MessageId", "ReceiptHandle", "MD5OfBody", "ApproximateFirstReceiveTimestamp", "SentTimestamp", "ResultMetadata",
	)).Assert(t.Name()+"_receive", recvOut)
}

func TestCLI_SQS_GetQueueUrlNotFound(t *testing.T) {
	_, stderr, err := runCLIExpectError(t, "sqs", "get-queue-url", "--queue-name", "nonexistent-cli-queue")
	if err == nil {
		t.Fatalf("expected kumo CLI to fail for a nonexistent queue, stderr:\n%s", stderr)
	}

	if len(stderr) == 0 {
		t.Fatal("expected stderr output describing the failure")
	}
}

func TestCLI_Kinesis_StreamLifecycle(t *testing.T) {
	streamName := "test-cli-kinesis-stream"

	runCLI(t, "kinesis", "create-stream", "--stream-name", streamName, "--shard-count", "1")

	client := newKinesisClient(t)
	t.Cleanup(func() {
		_, _ = client.DeleteStream(context.Background(), &kinesis.DeleteStreamInput{
			StreamName: aws.String(streamName),
		})
	})

	data := base64.StdEncoding.EncodeToString([]byte("Hello, CLI Kinesis!"))

	putOut := runCLI(t, "kinesis", "put-record", "--stream-name", streamName, "--data", data, "--partition-key", "partition-1")
	put := decodeCLIJSON(t, putOut)
	golden.New(t, golden.WithIgnoreFields("SequenceNumber", "ResultMetadata")).Assert(t.Name()+"_put", putOut)

	shardID, _ := put["ShardId"].(string)
	if shardID == "" {
		t.Fatalf("put-record: no ShardId in output: %s", putOut)
	}

	iterOut := runCLI(t, "kinesis", "get-shard-iterator",
		"--stream-name", streamName, "--shard-id", shardID, "--shard-iterator-type", "TRIM_HORIZON")
	iter := decodeCLIJSON(t, iterOut)

	shardIterator, _ := iter["ShardIterator"].(string)
	if shardIterator == "" {
		t.Fatalf("get-shard-iterator: no ShardIterator in output: %s", iterOut)
	}

	recordsOut := runCLI(t, "kinesis", "get-records", "--shard-iterator", shardIterator)
	golden.New(t, golden.WithIgnoreFields(
		"NextShardIterator", "SequenceNumber", "ApproximateArrivalTimestamp", "ResultMetadata",
	)).Assert(t.Name()+"_records", recordsOut)
}

func TestCLI_KMS_EncryptDecryptRoundtrip(t *testing.T) {
	createOut := runCLI(t, "kms", "create-key", "--description", "CLI test key", "--key-usage", "ENCRYPT_DECRYPT")
	created := decodeCLIJSON(t, createOut)
	golden.New(t, golden.WithIgnoreFields(
		"KeyId", "Arn", "CreationDate", "ResultMetadata",
	)).Assert(t.Name()+"_create", createOut)

	keyMetadata, _ := created["KeyMetadata"].(map[string]any)

	keyID, _ := keyMetadata["KeyId"].(string)
	if keyID == "" {
		t.Fatalf("create-key: no KeyMetadata.KeyId in output: %s", createOut)
	}

	client := newKMSClient(t)
	t.Cleanup(func() {
		_, _ = client.ScheduleKeyDeletion(context.Background(), &kms.ScheduleKeyDeletionInput{
			KeyId:               aws.String(keyID),
			PendingWindowInDays: aws.Int32(7),
		})
	})

	plaintext := "Hello, CLI KMS!"
	plaintextB64 := base64.StdEncoding.EncodeToString([]byte(plaintext))

	encryptOut := runCLI(t, "kms", "encrypt", "--key-id", keyID, "--plaintext", plaintextB64)
	encrypted := decodeCLIJSON(t, encryptOut)

	ciphertextB64, _ := encrypted["CiphertextBlob"].(string)
	if ciphertextB64 == "" {
		t.Fatalf("encrypt: no CiphertextBlob in output: %s", encryptOut)
	}

	decryptOut := runCLI(t, "kms", "decrypt", "--ciphertext-blob", ciphertextB64)
	decrypted := decodeCLIJSON(t, decryptOut)

	gotPlaintextB64, _ := decrypted["Plaintext"].(string)

	gotPlaintext, err := base64.StdEncoding.DecodeString(gotPlaintextB64)
	if err != nil {
		t.Fatalf("decode decrypted plaintext: %v", err)
	}

	if string(gotPlaintext) != plaintext {
		t.Errorf("plaintext mismatch: got %q, want %q", gotPlaintext, plaintext)
	}
}

func TestCLI_DynamoDB_CreateTableAndListTables(t *testing.T) {
	tableName := "test-cli-dynamodb-table"

	createOut := runCLI(t, "dynamodb", "create-table",
		"--table-name", tableName,
		"--attribute-definitions", "attribute-name=pk,attribute-type=S",
		"--key-schema", "attribute-name=pk,key-type=HASH",
		"--billing-mode", "PAY_PER_REQUEST",
	)
	golden.New(t, golden.WithIgnoreFields(
		"TableArn", "TableId", "CreationDateTime", "ResultMetadata",
	)).Assert(t.Name()+"_create", createOut)

	client := newDynamoDBClient(t)
	t.Cleanup(func() {
		_, _ = client.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{
			TableName: aws.String(tableName),
		})
	})

	listOut := runCLI(t, "dynamodb", "list-tables")
	list := decodeCLIJSON(t, listOut)

	names, _ := list["TableNames"].([]any)

	found := false

	for _, n := range names {
		if s, ok := n.(string); ok && s == tableName {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("table %s not found in list-tables output: %s", tableName, listOut)
	}
}

func TestCLI_Events_CreateBusAndPutEvents(t *testing.T) {
	busName := "test-cli-events-bus"

	createOut := runCLI(t, "events", "create-event-bus", "--name", busName)
	golden.New(t, golden.WithIgnoreFields("EventBusArn", "ResultMetadata")).Assert(t.Name()+"_create", createOut)

	client := newEventBridgeClient(t)
	t.Cleanup(func() {
		_, _ = client.DeleteEventBus(context.Background(), &eventbridge.DeleteEventBusInput{
			Name: aws.String(busName),
		})
	})

	entries := `[{"Source":"cli.test","DetailType":"cli-event","Detail":"{\"key\":\"value\"}","EventBusName":"` + busName + `"}]`

	putOut := runCLI(t, "events", "put-events", "--entries", entries)
	golden.New(t, golden.WithIgnoreFields("EventId", "ResultMetadata")).Assert(t.Name()+"_put", putOut)
}

func TestCLI_ACM_RequestAndDescribeCertificate(t *testing.T) {
	domainName := "cli-test.example.com"

	requestOut := runCLI(t, "acm", "request-certificate", "--domain-name", domainName)
	requested := decodeCLIJSON(t, requestOut)
	golden.New(t, golden.WithIgnoreFields("CertificateArn", "ResultMetadata")).Assert(t.Name()+"_request", requestOut)

	arn, _ := requested["CertificateArn"].(string)
	if arn == "" {
		t.Fatalf("request-certificate: no CertificateArn in output: %s", requestOut)
	}

	client := newACMClient(t)
	t.Cleanup(func() {
		_, _ = client.DeleteCertificate(context.Background(), &acm.DeleteCertificateInput{
			CertificateArn: aws.String(arn),
		})
	})

	describeOut := runCLI(t, "acm", "describe-certificate", "--certificate-arn", arn)
	golden.New(t, golden.WithIgnoreFields(
		"CertificateArn", "CreatedAt", "IssuedAt", "Serial", "Value", "ResultMetadata",
	)).Assert(t.Name()+"_describe", describeOut)
}
