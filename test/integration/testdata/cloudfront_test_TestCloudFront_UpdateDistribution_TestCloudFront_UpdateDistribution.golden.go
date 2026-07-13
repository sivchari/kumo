{
  "Distribution": {
    "ARN": "arn:aws:cloudfront::000000000000:distribution/E53f71198-7d73",
    "DistributionConfig": {
      "CallerReference": "test-update-distribution",
      "Comment": "Updated comment",
      "DefaultCacheBehavior": {
        "TargetOriginId": "myS3Origin",
        "ViewerProtocolPolicy": "allow-all",
        "AllowedMethods": null,
        "CachePolicyId": "658327ea-f89d-4fab-a63d-7e88639e58f6",
        "Compress": null,
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
        "ResponseHeadersPolicyId": null,
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
            "DomainName": "mybucket.s3.amazonaws.com",
            "Id": "myS3Origin",
            "ConnectionAttempts": null,
            "ConnectionTimeout": null,
            "CustomHeaders": null,
            "CustomOriginConfig": null,
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
      "Aliases": null,
      "AnycastIpListId": null,
      "CacheBehaviors": null,
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
    "DomainName": "E53f71198-7d73.cloudfront.net",
    "Id": "E53f71198-7d73",
    "InProgressInvalidationBatches": null,
    "LastModifiedTime": "2026-06-12T16:37:13+09:00",
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
  "ETag": "E50d50f90-9222-43b3-8e54-1034de06",
  "ResultMetadata": {}
}