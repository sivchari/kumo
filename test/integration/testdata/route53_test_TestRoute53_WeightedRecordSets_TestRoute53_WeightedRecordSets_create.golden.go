{
  "IsTruncated": false,
  "MaxItems": 100,
  "NextRecordIdentifier": null,
  "NextRecordName": null,
  "NextRecordType": "",
  "ResourceRecordSets": [
    {
      "AliasTarget": null,
      "CidrRoutingConfig": null,
      "Failover": "",
      "GeoLocation": null,
      "GeoProximityLocation": null,
      "HealthCheckId": null,
      "MultiValueAnswer": null,
      "Name": "www.{zone}.",
      "Region": "",
      "ResourceRecords": [
        {
          "Value": "192.0.2.1"
        }
      ],
      "SetIdentifier": "blue",
      "TTL": 60,
      "TrafficPolicyInstanceId": null,
      "Type": "A",
      "Weight": 10
    },
    {
      "AliasTarget": null,
      "CidrRoutingConfig": null,
      "Failover": "",
      "GeoLocation": null,
      "GeoProximityLocation": null,
      "HealthCheckId": null,
      "MultiValueAnswer": null,
      "Name": "www.{zone}.",
      "Region": "",
      "ResourceRecords": [
        {
          "Value": "192.0.2.2"
        }
      ],
      "SetIdentifier": "green",
      "TTL": 60,
      "TrafficPolicyInstanceId": null,
      "Type": "A",
      "Weight": 90
    }
  ],
  "ResultMetadata": {}
}