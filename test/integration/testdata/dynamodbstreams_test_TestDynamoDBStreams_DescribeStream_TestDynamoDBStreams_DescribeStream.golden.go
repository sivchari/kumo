{
  "StreamDescription": {
    "CreationRequestDateTime": "2026-05-18T15:45:55Z",
    "KeySchema": [
      {
        "AttributeName": "pk",
        "KeyType": "HASH"
      }
    ],
    "LastEvaluatedShardId": null,
    "Shards": [
      {
        "ParentShardId": null,
        "SequenceNumberRange": {
          "EndingSequenceNumber": null,
          "StartingSequenceNumber": "000000000000000000001"
        },
        "ShardId": "shardId-000000000000"
      }
    ],
    "StreamArn": "arn:aws:dynamodb:us-east-1:000000000000:table/test-streams-describe/stream/2026-05-19T00:45:55.479",
    "StreamLabel": "2026-05-19T00:45:55.479",
    "StreamStatus": "ENABLED",
    "StreamViewType": "NEW_AND_OLD_IMAGES",
    "TableName": "test-streams-describe"
  },
  "ResultMetadata": {}
}