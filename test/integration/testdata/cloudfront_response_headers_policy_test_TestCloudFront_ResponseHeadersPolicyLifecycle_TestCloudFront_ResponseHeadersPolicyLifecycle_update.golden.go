{
  "ETag": "E17c291ea-8901-4482-9251-4b0175aa",
  "ResponseHeadersPolicy": {
    "Id": "819b3a36-9469-4b2a-bdcf-acedb7ea324a",
    "LastModifiedTime": "2026-10-07T23:38:52Z",
    "ResponseHeadersPolicyConfig": {
      "Name": "test-cf-rhp-lifecycle-dlz03a09ou4n",
      "Comment": "updated",
      "CorsConfig": null,
      "CustomHeadersConfig": {
        "Quantity": 1,
        "Items": [
          {
            "Header": "X-Updated",
            "Override": true,
            "Value": "yes"
          }
        ]
      },
      "RemoveHeadersConfig": null,
      "SecurityHeadersConfig": {
        "ContentSecurityPolicy": null,
        "ContentTypeOptions": null,
        "FrameOptions": null,
        "ReferrerPolicy": null,
        "StrictTransportSecurity": null,
        "XSSProtection": {
          "Override": true,
          "Protection": false,
          "ModeBlock": null,
          "ReportUri": null
        }
      },
      "ServerTimingHeadersConfig": {
        "Enabled": false,
        "SamplingRate": null
      }
    }
  },
  "ResultMetadata": {}
}