{
  "Attributes": {
    "ApproximateNumberOfMessages": "0",
    "ApproximateNumberOfMessagesNotVisible": "0",
    "ContentBasedDeduplication": "false",
    "CreatedTimestamp": "1778591038",
    "DelaySeconds": "0",
    "FifoQueue": "false",
    "LastModifiedTimestamp": "1778591038",
    "MaximumMessageSize": "262144",
    "MessageRetentionPeriod": "345600",
    "Policy": "{\"Version\":\"2012-10-17\",\"Statement\":[{\"Sid\":\"AllowSNS\",\"Effect\":\"Allow\",\"Principal\":{\"Service\":\"sns.amazonaws.com\"},\"Action\":\"sqs:SendMessage\",\"Resource\":\"*\"}]}",
    "QueueArn": "arn:aws:sqs:us-east-1:000000000000:test-queue-policy",
    "ReceiveMessageWaitTimeSeconds": "0",
    "VisibilityTimeout": "30"
  },
  "ResultMetadata": {}
}