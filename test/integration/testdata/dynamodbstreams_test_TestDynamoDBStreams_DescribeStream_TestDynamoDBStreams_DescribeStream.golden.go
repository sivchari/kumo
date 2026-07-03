{
  "StreamDescription": {
    "CreationRequestDateTime": "2026-05-19T02:24:12Z",
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
    "StreamArn": "arn:aws:dynamodb:us-east-1:000000000000:table/test-streams-describe/stream/2026-05-19T11:24:12.960",
    "StreamLabel": "2026-05-19T11:24:12.960",
    "StreamStatus": "ENABLED",
    "StreamViewType": "NEW_AND_OLD_IMAGES",
    "TableName": "test-streams-describe"
  },
  "ResultMetadata": {}
}