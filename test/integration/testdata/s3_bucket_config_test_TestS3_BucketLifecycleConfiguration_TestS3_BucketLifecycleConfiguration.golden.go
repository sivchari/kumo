{
  "Rules": [
    {
      "Status": "Enabled",
      "AbortIncompleteMultipartUpload": null,
      "Expiration": {
        "Date": null,
        "Days": 30,
        "ExpiredObjectDeleteMarker": null
      },
      "Filter": {
        "And": null,
        "ObjectSizeGreaterThan": null,
        "ObjectSizeLessThan": null,
        "Prefix": "logs/",
        "Tag": null
      },
      "ID": "expire-logs",
      "NoncurrentVersionExpiration": null,
      "NoncurrentVersionTransitions": null,
      "Prefix": null,
      "Transitions": null
    }
  ],
  "TransitionDefaultMinimumObjectSize": "",
  "ResultMetadata": {}
}