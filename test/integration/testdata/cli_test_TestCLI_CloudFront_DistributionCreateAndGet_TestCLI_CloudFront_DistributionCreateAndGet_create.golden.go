{
  "Distribution": {
    "ARN": "arn:aws:cloudfront::000000000000:distribution/Eea728034-b4d7",
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
      "CacheBehaviors": null,
      "CacheTagConfig": null,
      "CallerReference": "test-cli-cloudfront-create-distribution",
      "Comment": "CLI test distribution",
      "ConnectionFunctionAssociation": null,
      "ConnectionMode": "",
      "ContinuousDeploymentPolicyId": null,
      "CustomErrorResponses": null,
      "DefaultCacheBehavior": {
        "AllowedMethods": null,
        "CachePolicyId": "658327ea-f89d-4fab-a63d-7e88639e58f6",
        "Compress": null,
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
        "ResponseHeadersPolicyId": null,
        "SmoothStreaming": null,
        "TargetOriginId": "myS3Origin",
        "TrustedKeyGroups": null,
        "TrustedSigners": null,
        "ViewerProtocolPolicy": "allow-all"
      },
      "DefaultRootObject": null,
      "Enabled": true,
      "HttpVersion": "http2",
      "IsIPV6Enabled": null,
      "Logging": null,
      "OriginGroups": null,
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
    "DomainName": "Eea728034-b4d7.cloudfront.net",
    "Id": "Eea728034-b4d7",
    "InProgressInvalidationBatches": null,
    "LastModifiedTime": "2026-08-06T18:06:55+09:00",
    "Status": "InProgress"
  },
  "ETag": "E42557c67-6c26-4e11-bb14-fc3cb426",
  "Location": "/2020-05-31/distribution/Eea728034-b4d7",
  "ResultMetadata": {}
}