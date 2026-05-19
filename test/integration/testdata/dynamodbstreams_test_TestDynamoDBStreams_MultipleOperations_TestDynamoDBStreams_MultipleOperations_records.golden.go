{
  "NextShardIterator": "YXJuOmF3czpkeW5hbW9kYjp1cy1lYXN0LTE6MDAwMDAwMDAwMDAwOnRhYmxlL3Rlc3Qtc3RyZWFtcy1tdWx0aS1vcHMvc3RyZWFtLzIwMjYtMDUtMTlUMTE6MjQ6MTMuMDcxOnNoYXJkSWQtMDAwMDAwMDAwMDAwOjM6MTc3OTE1NzQ1MzE3NDk1ODAwMA==",
  "Records": [
    {
      "AwsRegion": "us-east-1",
      "Dynamodb": {
        "ApproximateCreationDateTime": "2026-05-19T02:24:13Z",
        "Keys": {
          "pk": {
            "Value": "multi-1"
          }
        },
        "NewImage": {
          "data": {
            "Value": "original"
          },
          "pk": {
            "Value": "multi-1"
          }
        },
        "OldImage": null,
        "SequenceNumber": "000000000000000000005",
        "SizeBytes": 100,
        "StreamViewType": "NEW_AND_OLD_IMAGES"
      },
      "EventID": "ec5d7692-a5fa-4594-8110-2e9d4219d755",
      "EventName": "INSERT",
      "EventSource": "aws:dynamodb",
      "EventVersion": "1.1",
      "UserIdentity": null
    },
    {
      "AwsRegion": "us-east-1",
      "Dynamodb": {
        "ApproximateCreationDateTime": "2026-05-19T02:24:13Z",
        "Keys": {
          "pk": {
            "Value": "multi-1"
          }
        },
        "NewImage": {
          "data": {
            "Value": "updated"
          },
          "pk": {
            "Value": "multi-1"
          }
        },
        "OldImage": {
          "data": {
            "Value": "original"
          },
          "pk": {
            "Value": "multi-1"
          }
        },
        "SequenceNumber": "000000000000000000006",
        "SizeBytes": 100,
        "StreamViewType": "NEW_AND_OLD_IMAGES"
      },
      "EventID": "ac8f7794-75c1-44e9-bd7d-497b73343b91",
      "EventName": "MODIFY",
      "EventSource": "aws:dynamodb",
      "EventVersion": "1.1",
      "UserIdentity": null
    },
    {
      "AwsRegion": "us-east-1",
      "Dynamodb": {
        "ApproximateCreationDateTime": "2026-05-19T02:24:13Z",
        "Keys": {
          "pk": {
            "Value": "multi-1"
          }
        },
        "NewImage": null,
        "OldImage": {
          "data": {
            "Value": "updated"
          },
          "pk": {
            "Value": "multi-1"
          }
        },
        "SequenceNumber": "000000000000000000007",
        "SizeBytes": 100,
        "StreamViewType": "NEW_AND_OLD_IMAGES"
      },
      "EventID": "2aa3cf53-442f-416d-845f-e04d692420cb",
      "EventName": "REMOVE",
      "EventSource": "aws:dynamodb",
      "EventVersion": "1.1",
      "UserIdentity": null
    }
  ],
  "ResultMetadata": {}
}