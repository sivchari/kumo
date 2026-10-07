{
  "Distribution": {
    "ARN": "arn:aws:cloudfront::000000000000:distribution/E210e74d4-d37e",
    "DistributionConfig": {
      "CallerReference": "test-create-distribution",
      "Comment": "Test distribution",
      "DefaultCacheBehavior": {
        "TargetOriginId": "myS3Origin",
        "ViewerProtocolPolicy": "allow-all",
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
      "CacheBehaviors": {
        "Quantity": 0,
        "Items": []
      },
      "ConnectionFunctionAssociation": null,
      "ConnectionMode": "",
      "ContinuousDeploymentPolicyId": null,
      "CustomErrorResponses": null,
      "DefaultRootObject": null,
      "HttpVersion": "http2",
      "IsIPV6Enabled": null,
      "Logging": {
        "Bucket": "",
        "Enabled": false,
        "IncludeCookies": false,
        "Prefix": ""
      },
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
    "DomainName": "E210e74d4-d37e.cloudfront.net",
    "Id": "E210e74d4-d37e",
    "InProgressInvalidationBatches": null,
    "LastModifiedTime": "2026-10-08T07:28:45+09:00",
    "Status": "Deployed",
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
  "ETag": "E4b279483-9c48-4a8e-a067-1b5be575",
  "Location": "/2020-05-31/distribution/E210e74d4-d37e",
  "ResultMetadata": {}
}