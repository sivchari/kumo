{
  "ETag": "E1cf91973-e35a-4b34-a681-f7d34468",
  "ResponseHeadersPolicy": {
    "Id": "819b3a36-9469-4b2a-bdcf-acedb7ea324a",
    "LastModifiedTime": "2026-10-07T23:38:52Z",
    "ResponseHeadersPolicyConfig": {
      "Name": "test-cf-rhp-lifecycle-dlz03a09ou4n",
      "Comment": "kumo integration test",
      "CorsConfig": {
        "AccessControlAllowCredentials": false,
        "AccessControlAllowHeaders": {
          "Items": [
            "Authorization",
            "Content-Type"
          ],
          "Quantity": 2
        },
        "AccessControlAllowMethods": {
          "Items": [
            "GET",
            "OPTIONS"
          ],
          "Quantity": 2
        },
        "AccessControlAllowOrigins": {
          "Items": [
            "https://www.example.com"
          ],
          "Quantity": 1
        },
        "OriginOverride": true,
        "AccessControlExposeHeaders": {
          "Quantity": 1,
          "Items": [
            "X-Request-Id"
          ]
        },
        "AccessControlMaxAgeSec": 0
      },
      "CustomHeadersConfig": {
        "Quantity": 2,
        "Items": [
          {
            "Header": "X-Kumo",
            "Override": false,
            "Value": "1"
          },
          {
            "Header": "X-Empty",
            "Override": true,
            "Value": ""
          }
        ]
      },
      "RemoveHeadersConfig": {
        "Quantity": 1,
        "Items": [
          {
            "Header": "X-Powered-By"
          }
        ]
      },
      "SecurityHeadersConfig": {
        "ContentSecurityPolicy": {
          "ContentSecurityPolicy": "default-src 'self'",
          "Override": true
        },
        "ContentTypeOptions": {
          "Override": false
        },
        "FrameOptions": {
          "FrameOption": "SAMEORIGIN",
          "Override": true
        },
        "ReferrerPolicy": {
          "Override": false,
          "ReferrerPolicy": "strict-origin-when-cross-origin"
        },
        "StrictTransportSecurity": {
          "AccessControlMaxAgeSec": 31536000,
          "Override": true,
          "IncludeSubdomains": true,
          "Preload": false
        },
        "XSSProtection": {
          "Override": false,
          "Protection": true,
          "ModeBlock": false,
          "ReportUri": "https://www.example.com/xss"
        }
      },
      "ServerTimingHeadersConfig": {
        "Enabled": true,
        "SamplingRate": 12.5
      }
    }
  },
  "ResultMetadata": {}
}