{
  "NextShardIterator": "YXJuOmF3czpkeW5hbW9kYjp1cy1lYXN0LTE6MDAwMDAwMDAwMDAwOnRhYmxlL3Rlc3Qtc3RyZWFtcy1yZWNvcmRzL3N0cmVhbS8yMDI2LTA1LTE5VDExOjI0OjEyLjk2NDpzaGFyZElkLTAwMDAwMDAwMDAwMDoxOjE3NzkxNTc0NTMwNjg1MzgwMDA=",
  "Records": [
    {
      "AwsRegion": "us-east-1",
      "Dynamodb": {
        "ApproximateCreationDateTime": "2026-05-19T02:24:12Z",
        "Keys": {
          "pk": {
            "Value": "stream-item-1"
          }
        },
        "NewImage": {
          "data": {
            "Value": "hello"
          },
          "pk": {
            "Value": "stream-item-1"
          }
        },
        "OldImage": null,
        "SequenceNumber": "000000000000000000003",
        "SizeBytes": 100,
        "StreamViewType": "NEW_AND_OLD_IMAGES"
      },
      "EventID": "09b68334-456d-45e6-81a6-e192ccbb3587",
      "EventName": "INSERT",
      "EventSource": "aws:dynamodb",
      "EventVersion": "1.1",
      "UserIdentity": null
    }
  ],
  "ResultMetadata": {}
}