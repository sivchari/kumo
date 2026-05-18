{
  "NextShardIterator": "YXJuOmF3czpkeW5hbW9kYjp1cy1lYXN0LTE6MDAwMDAwMDAwMDAwOnRhYmxlL3Rlc3Qtc3RyZWFtcy1yZWNvcmRzL3N0cmVhbS8yMDI2LTA1LTE5VDAwOjQ1OjU1LjQ4MzpzaGFyZElkLTAwMDAwMDAwMDAwMDoxOjE3NzkxMTkxNTU1ODcwODgwMDA=",
  "Records": [
    {
      "AwsRegion": "us-east-1",
      "Dynamodb": {
        "ApproximateCreationDateTime": "2026-05-18T15:45:55Z",
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
      "EventID": "6bf2a4e8-cb18-4e56-8b94-de762eb3c90f",
      "EventName": "INSERT",
      "EventSource": "aws:dynamodb",
      "EventVersion": "1.1",
      "UserIdentity": null
    }
  ],
  "ResultMetadata": {}
}