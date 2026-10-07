{
  "CachePolicyConfig": {
    "MinTTL": 0,
    "Name": "test-cf-cache-policy-lifecycle-dlyymy0oubg7",
    "Comment": "kumo integration test",
    "DefaultTTL": 3600,
    "MaxTTL": 86400,
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
  "ETag": "E0ab27869-abad-4cbb-b1d2-2cbb5916",
  "ResultMetadata": {}
}