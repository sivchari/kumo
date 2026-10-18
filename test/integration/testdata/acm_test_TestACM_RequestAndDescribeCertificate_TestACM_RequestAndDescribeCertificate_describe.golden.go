{
  "Certificate": {
    "CertificateArn": "arn:aws:acm:us-east-1:000000000000:certificate/83cdbf97-467c-47bc-9090-534509209c6e",
    "CertificateAuthorityArn": null,
    "CreatedAt": "2026-03-23T07:45:24.794Z",
    "DomainName": "example.com",
    "DomainValidationOptions": [
      {
        "DomainName": "example.com",
        "HttpRedirect": null,
        "ResourceRecord": {
          "Name": "_acme-challenge.example.com",
          "Type": "CNAME",
          "Value": "_83cdbf97.acm-validations.aws."
        },
        "ValidationDomain": "example.com",
        "ValidationEmails": null,
        "ValidationMethod": "DNS",
        "ValidationStatus": "SUCCESS"
      },
      {
        "DomainName": "www.example.com",
        "HttpRedirect": null,
        "ResourceRecord": {
          "Name": "_acme-challenge.www.example.com",
          "Type": "CNAME",
          "Value": "_83cdbf97.acm-validations.aws."
        },
        "ValidationDomain": "www.example.com",
        "ValidationEmails": null,
        "ValidationMethod": "DNS",
        "ValidationStatus": "SUCCESS"
      },
      {
        "DomainName": "api.example.com",
        "HttpRedirect": null,
        "ResourceRecord": {
          "Name": "_acme-challenge.api.example.com",
          "Type": "CNAME",
          "Value": "_83cdbf97.acm-validations.aws."
        },
        "ValidationDomain": "api.example.com",
        "ValidationEmails": null,
        "ValidationMethod": "DNS",
        "ValidationStatus": "SUCCESS"
      }
    ],
    "ExtendedKeyUsages": null,
    "FailureReason": "",
    "ImportedAt": null,
    "InUseBy": null,
    "IssuedAt": "2026-03-23T07:45:24.794Z",
    "Issuer": null,
    "KeyAlgorithm": "RSA_2048",
    "KeyUsages": null,
    "ManagedBy": "",
    "NotAfter": null,
    "NotBefore": null,
    "Options": null,
    "RenewalEligibility": "INELIGIBLE",
    "RenewalSummary": null,
    "RevocationReason": "",
    "RevokedAt": null,
    "Serial": "20a3ff8d59956a02d70df89a6509c41c",
    "SignatureAlgorithm": null,
    "Status": "ISSUED",
    "Subject": "CN=example.com",
    "SubjectAlternativeNames": [
      "www.example.com",
      "api.example.com"
    ],
    "Type": "AMAZON_ISSUED"
  },
  "ResultMetadata": {}
}