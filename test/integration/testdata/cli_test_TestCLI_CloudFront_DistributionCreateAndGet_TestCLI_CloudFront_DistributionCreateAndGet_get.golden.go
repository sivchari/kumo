{
  "Distribution": {
    "ARN": "arn:aws:cloudfront::000000000000:distribution/E5738e109-a3ba",
    "ActiveTrustedKeyGroups": {
      "Enabled": false,
      "Items": null,
      "Quantity": 0
    },
    "ActiveTrustedSigners": {
      "Enabled": false,
      "Items": null,
      "Quantity": 0
    },
    "AliasICPRecordals": null,
    "DistributionConfig": {
      "Aliases": null,
      "AnycastIpListId": null,
      "CacheBehaviors": {
        "Items": [],
        "Quantity": 0
      },
      "CacheTagConfig": null,
      "CallerReference": "test-cli-cloudfront-create-distribution",
      "Comment": "CLI test distribution",
      "ConnectionFunctionAssociation": null,
      "ConnectionMode": "",
      "ContinuousDeploymentPolicyId": null,
      "CustomErrorResponses": null,
      "DefaultCacheBehavior": {
        "AllowedMethods": {
          "CachedMethods": {
            "Items": [
              "GET",
              "HEAD"
            ],
            "Quantity": 2
          },
          "Items": [
            "GET",
            "HEAD"
          ],
          "Quantity": 2
        },
        "CachePolicyId": "658327ea-f89d-4fab-a63d-7e88639e58f6",
        "Compress": null,
        "DefaultTTL": null,
        "FieldLevelEncryptionId": null,
        "ForwardedValues": null,
        "FunctionAssociations": {
          "Items": null,
          "Quantity": 0
        },
        "GrpcConfig": null,
        "LambdaFunctionAssociations": {
          "Items": null,
          "Quantity": 0
        },
        "MaxTTL": null,
        "MinTTL": null,
        "OriginRequestPolicyId": null,
        "RealtimeLogConfigArn": null,
        "ResponseHeadersPolicyId": null,
        "SmoothStreaming": null,
        "TargetOriginId": "myS3Origin",
        "TrustedKeyGroups": {
          "Enabled": false,
          "Items": [],
          "Quantity": 0
        },
        "TrustedSigners": {
          "Enabled": false,
          "Items": [],
          "Quantity": 0
        },
        "ViewerProtocolPolicy": "allow-all"
      },
      "DefaultRootObject": null,
      "Enabled": true,
      "HttpVersion": "http2",
      "IsIPV6Enabled": null,
      "Logging": {
        "Bucket": "",
        "Enabled": false,
        "IncludeCookies": false,
        "Prefix": ""
      },
      "OriginGroups": {
        "Items": null,
        "Quantity": 0
      },
      "Origins": {
        "Items": [
          {
            "ConnectionAttempts": null,
            "ConnectionTimeout": null,
            "CustomHeaders": null,
            "CustomOriginConfig": null,
            "DomainName": "mybucket.s3.amazonaws.com",
            "Id": "myS3Origin",
            "OriginAccessControlId": null,
            "OriginPath": null,
            "OriginShield": null,
            "ResponseCompletionTimeout": null,
            "S3OriginConfig": {
              "OriginAccessIdentity": "",
              "OriginReadTimeout": null
            },
            "VpcOriginConfig": null
          }
        ],
        "Quantity": 1
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
    "DomainName": "E5738e109-a3ba.cloudfront.net",
    "Id": "E5738e109-a3ba",
    "InProgressInvalidationBatches": null,
    "LastModifiedTime": "2026-10-08T07:29:16+09:00",
    "Status": "Deployed"
  },
  "ETag": "E30e68387-835e-4e61-85ad-6f5d2c2a",
  "ResultMetadata": {}
}