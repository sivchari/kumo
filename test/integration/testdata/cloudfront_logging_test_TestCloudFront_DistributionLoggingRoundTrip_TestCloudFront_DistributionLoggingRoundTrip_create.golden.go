{
  "Distribution": {
    "ARN": "arn:aws:cloudfront::000000000000:distribution/E931abe37-80a5",
    "DistributionConfig": {
      "CallerReference": "test-cf-logging-dmd5wizdg7iy",
      "Comment": "distribution logging",
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
        "CachePolicyId": "4135ea2d-6df8-44a3-9df3-4b5a84be39ad",
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
      "CustomErrorResponses": {
        "Quantity": 0,
        "Items": null
      },
      "DefaultRootObject": null,
      "HttpVersion": "http2",
      "IsIPV6Enabled": null,
      "Logging": {
        "Bucket": "cf-logs.s3.amazonaws.com",
        "Enabled": true,
        "IncludeCookies": false,
        "Prefix": "initial/"
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
    "DomainName": "E931abe37-80a5.cloudfront.net",
    "Id": "E931abe37-80a5",
    "InProgressInvalidationBatches": null,
    "LastModifiedTime": "2026-10-25T00:09:29+09:00",
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
  "ETag": "Eba5776e3-0a9c-4dfb-8476-8c468188",
  "Location": "/2020-05-31/distribution/E931abe37-80a5",
  "ResultMetadata": {}
}