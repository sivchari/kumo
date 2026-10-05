{
  "Distribution": {
    "ARN": "arn:aws:cloudfront::000000000000:distribution/E5bf3f1ea-046f",
    "DistributionConfig": {
      "CallerReference": "test-cf-cache-behaviors-dlwpkqszzdts",
      "Comment": "ordered cache behaviors",
      "DefaultCacheBehavior": {
        "TargetOriginId": "assets",
        "ViewerProtocolPolicy": "redirect-to-https",
        "AllowedMethods": {
          "Items": [
            "GET",
            "HEAD"
          ],
          "Quantity": 2,
          "CachedMethods": {
            "Items": [
              "GET",
              "HEAD"
            ],
            "Quantity": 2
          }
        },
        "CachePolicyId": "658327ea-f89d-4fab-a63d-7e88639e58f6",
        "Compress": true,
        "DefaultTTL": null,
        "FieldLevelEncryptionId": null,
        "ForwardedValues": null,
        "FunctionAssociations": {
          "Quantity": 0,
          "Items": null
        },
        "GrpcConfig": null,
        "LambdaFunctionAssociations": {
          "Quantity": 0,
          "Items": null
        },
        "MaxTTL": null,
        "MinTTL": null,
        "OriginRequestPolicyId": null,
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
        "Quantity": 1,
        "Items": [
          {
            "PathPattern": "/v2/*",
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
            "FunctionAssociations": {
              "Quantity": 0,
              "Items": null
            },
            "GrpcConfig": null,
            "LambdaFunctionAssociations": {
              "Quantity": 0,
              "Items": null
            },
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
      "OriginGroups": {
        "Quantity": 0,
        "Items": null
      },
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
    "DomainName": "E5bf3f1ea-046f.cloudfront.net",
    "Id": "E5bf3f1ea-046f",
    "InProgressInvalidationBatches": null,
    "LastModifiedTime": "2026-10-05T15:59:09+09:00",
    "Status": "InProgress",
    "ActiveTrustedKeyGroups": {
      "Enabled": false,
      "Quantity": 0,
      "Items": null
    },
    "ActiveTrustedSigners": {
      "Enabled": false,
      "Quantity": 0,
      "Items": null
    },
    "AliasICPRecordals": null
  },
  "ETag": "E75f1317c-6a06-485c-8ddd-10adb9ad",
  "ResultMetadata": {}
}