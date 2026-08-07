{
  "Attributes": {
    "Policy": "{\"Version\":\"2012-10-17\",\"Id\":\"arn:aws:sqs:us-east-1:000000000000:test-queue-add-remove-permission/SQSDefaultPolicy\",\"Statement\":[{\"Action\":[\"SQS:SendMessage\",\"SQS:ReceiveMessage\"],\"Effect\":\"Allow\",\"Principal\":{\"AWS\":[\"arn:aws:iam::177715257436:root\",\"arn:aws:iam::111111111111:root\"]},\"Resource\":\"arn:aws:sqs:us-east-1:000000000000:test-queue-add-remove-permission\",\"Sid\":\"TestSharedAccess\"}]}"
  },
  "ResultMetadata": {}
}