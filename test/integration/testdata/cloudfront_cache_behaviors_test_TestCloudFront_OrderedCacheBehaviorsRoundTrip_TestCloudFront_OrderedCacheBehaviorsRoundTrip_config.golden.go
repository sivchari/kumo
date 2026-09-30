{
  "DistributionConfig": {
    "CallerReference": "test-cf-cache-behaviors-dlsyz1iujhau",
    "Comment": "ordered cache behaviors",
    "DefaultCacheBehavior": {
      "TargetOriginId": "assets",
      "ViewerProtocolPolicy": "redirect-to-https",
      "AllowedMethods": null,
      "CachePolicyId": "658327ea-f89d-4fab-a63d-7e88639e58f6",
      "Compress": true,
      "DefaultTTL": null,
      "FieldLevelEncryptionId": null,
      "ForwardedValues": null,
      "FunctionAssociations": null,
      "GrpcConfig": null,
      "LambdaFunctionAssociations": null,
      "MaxTTL": null,
      "MinTTL": null,
      "OriginRequestPolicyId": null,
      "RealtimeLogConfigArn": null,
      "ResponseHeadersPolicyId": "67f7725c-6f97-4210-82d7-5512b31e9d03",
      "SmoothStreaming": null,
      "TrustedKeyGroups": null,
      "TrustedSigners": null
    },
    "Enabled": true,
    "Origins": {
      "Items": [
        {
          "DomainName": "assets.example.com",
          "Id": "assets",
          "ConnectionAttempts": null,
          "ConnectionTimeout": null,
          "CustomHeaders": null,
          "CustomOriginConfig": {
            "HTTPPort": 80,
            "HTTPSPort": 443,
            "OriginProtocolPolicy": "https-only",
            "IpAddressType": "",
            "OriginKeepaliveTimeout": null,
            "OriginMtlsConfig": null,
            "OriginReadTimeout": null,
            "OriginSslProtocols": null
          },
          "OriginAccessControlId": null,
          "OriginPath": null,
          "OriginShield": null,
          "ResponseCompletionTimeout": null,
          "S3OriginConfig": null,
          "VpcOriginConfig": null
        },
        {
          "DomainName": "api.example.com",
          "Id": "api",
          "ConnectionAttempts": null,
          "ConnectionTimeout": null,
          "CustomHeaders": null,
          "CustomOriginConfig": {
            "HTTPPort": 80,
            "HTTPSPort": 443,
            "OriginProtocolPolicy": "https-only",
            "IpAddressType": "",
            "OriginKeepaliveTimeout": null,
            "OriginMtlsConfig": null,
            "OriginReadTimeout": null,
            "OriginSslProtocols": null
          },
          "OriginAccessControlId": null,
          "OriginPath": null,
          "OriginShield": null,
          "ResponseCompletionTimeout": null,
          "S3OriginConfig": null,
          "VpcOriginConfig": null
        }
      ],
      "Quantity": 2
    },
    "Aliases": null,
    "AnycastIpListId": null,
    "CacheBehaviors": {
      "Quantity": 2,
      "Items": [
        {
          "PathPattern": "/api/*",
          "TargetOriginId": "api",
          "ViewerProtocolPolicy": "redirect-to-https",
          "AllowedMethods": {
            "Items": [
              "GET",
              "HEAD",
              "OPTIONS"
            ],
            "Quantity": 3,
            "CachedMethods": {
              "Items": [
                "GET",
                "HEAD"
              ],
              "Quantity": 2
            }
          },
          "CachePolicyId": "4135ea2d-6df8-44a3-9df3-4b5a84be39ad",
          "Compress": true,
          "DefaultTTL": null,
          "FieldLevelEncryptionId": null,
          "ForwardedValues": null,
          "FunctionAssociations": null,
          "GrpcConfig": null,
          "LambdaFunctionAssociations": null,
          "MaxTTL": null,
          "MinTTL": null,
          "OriginRequestPolicyId": "b689b0a8-53d0-40ab-baf2-68738e2966ac",
          "RealtimeLogConfigArn": null,
          "ResponseHeadersPolicyId": "67f7725c-6f97-4210-82d7-5512b31e9d03",
          "SmoothStreaming": null,
          "TrustedKeyGroups": {
            "Enabled": false,
            "Quantity": 0,
            "Items": []
          },
          "TrustedSigners": {
            "Enabled": false,
            "Quantity": 0,
            "Items": []
          }
        },
        {
          "PathPattern": "/legacy/*",
          "TargetOriginId": "assets",
          "ViewerProtocolPolicy": "allow-all",
          "AllowedMethods": null,
          "CachePolicyId": null,
          "Compress": null,
          "DefaultTTL": 300,
          "FieldLevelEncryptionId": null,
          "ForwardedValues": {
            "Cookies": {
              "Forward": "whitelist",
              "WhitelistedNames": {
                "Quantity": 1,
                "Items": [
                  "session"
                ]
              }
            },
            "QueryString": true,
            "Headers": {
              "Quantity": 1,
              "Items": [
                "Authorization"
              ]
            },
            "QueryStringCacheKeys": {
              "Quantity": 1,
              "Items": [
                "v"
              ]
            }
          },
          "FunctionAssociations": null,
          "GrpcConfig": null,
          "LambdaFunctionAssociations": null,
          "MaxTTL": 3600,
          "MinTTL": null,
          "OriginRequestPolicyId": null,
          "RealtimeLogConfigArn": null,
          "ResponseHeadersPolicyId": null,
          "SmoothStreaming": null,
          "TrustedKeyGroups": null,
          "TrustedSigners": null
        }
      ]
    },
    "ConnectionFunctionAssociation": null,
    "ConnectionMode": "",
    "ContinuousDeploymentPolicyId": null,
    "CustomErrorResponses": null,
    "DefaultRootObject": null,
    "HttpVersion": "http2",
    "IsIPV6Enabled": null,
    "Logging": null,
    "OriginGroups": null,
    "PriceClass": "PriceClass_All",
    "Restrictions": null,
    "Staging": null,
    "TenantConfig": null,
    "ViewerCertificate": {
      "ACMCertificateArn": null,
      "Certificate": null,
      "CertificateSource": "",
      "CloudFrontDefaultCertificate": true,
      "IAMCertificateId": null,
      "MinimumProtocolVersion": "TLSv1",
      "SSLSupportMethod": ""
    },
    "ViewerMtlsConfig": null,
    "WebACLId": null
  },
  "ETag": "E38d78c53-bec5-4b0d-ab48-8d5afe13",
  "ResultMetadata": {}
}