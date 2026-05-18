{
  "NextShardIterator": "YXJuOmF3czpkeW5hbW9kYjp1cy1lYXN0LTE6MDAwMDAwMDAwMDAwOnRhYmxlL3Rlc3Qtc3RyZWFtcy1tdWx0aS1vcHMvc3RyZWFtLzIwMjYtMDUtMTlUMDA6NDU6NTUuNTg4OnNoYXJkSWQtMDAwMDAwMDAwMDAwOjM6MTc3OTExOTE1NTY5Mjk3MzAwMA==",
  "Records": [
    {
      "AwsRegion": "us-east-1",
      "Dynamodb": {
        "ApproximateCreationDateTime": "2026-05-18T15:45:55Z",
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
      "EventID": "f578ad21-f228-491a-8244-ff018c85d516",
      "EventName": "INSERT",
      "EventSource": "aws:dynamodb",
      "EventVersion": "1.1",
      "UserIdentity": null
    },
    {
      "AwsRegion": "us-east-1",
      "Dynamodb": {
        "ApproximateCreationDateTime": "2026-05-18T15:45:55Z",
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
      "EventID": "a4aee6c8-e2d5-42d7-84a2-79acd07a3f8a",
      "EventName": "MODIFY",
      "EventSource": "aws:dynamodb",
      "EventVersion": "1.1",
      "UserIdentity": null
    },
    {
      "AwsRegion": "us-east-1",
      "Dynamodb": {
        "ApproximateCreationDateTime": "2026-05-18T15:45:55Z",
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
      "EventID": "5b5be5a8-ab7d-4e17-aa23-7fcdc0474113",
      "EventName": "REMOVE",
      "EventSource": "aws:dynamodb",
      "EventVersion": "1.1",
      "UserIdentity": null
    }
  ],
  "ResultMetadata": {}
}