{
  "CachePolicy": {
    "CachePolicyConfig": {
      "MinTTL": 40000000,
      "Name": "test-cf-cache-policy-max-ttl-default-dmd671qv3mw5",
      "Comment": "kumo integration test",
      "DefaultTTL": 100,
      "MaxTTL": 100,
      "ParametersInCacheKeyAndForwardedToOrigin": {
        "CookiesConfig": {
          "CookieBehavior": "allExcept",
          "Cookies": {
            "Quantity": 1,
            "Items": [
              "tracking"
            ]
          }
        },
        "EnableAcceptEncodingGzip": false,
        "HeadersConfig": {
          "HeaderBehavior": "whitelist",
          "Headers": {
            "Quantity": 2,
            "Items": [
              "Origin",
              "Accept-Language"
            ]
          }
        },
        "QueryStringsConfig": {
          "QueryStringBehavior": "whitelist",
          "QueryStrings": {
            "Quantity": 1,
            "Items": [
              "v"
            ]
          }
        },
        "EnableAcceptEncodingBrotli": true
      }
    },
    "Id": "a897e5d9-7920-44f4-b6c6-a9bd472ad87c",
    "LastModifiedTime": "2026-10-24T15:23:14Z"
  },
  "ETag": "Ee5f97dea-16b6-4f05-8bc6-4137f1e3",
  "ResultMetadata": {}
}