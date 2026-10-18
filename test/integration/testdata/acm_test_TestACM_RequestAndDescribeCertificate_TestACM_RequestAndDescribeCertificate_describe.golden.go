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
          "Name": "_09962a7c6e9895cf51f7c722e052f151.example.com.",
          "Type": "CNAME",
          "Value": "_7b9657de66610d75e9b8838eb2825663.acm-validations.aws."
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
          "Name": "_bff56a53f67e97f790fe82075c77fc30.www.example.com.",
          "Type": "CNAME",
          "Value": "_3431e70bf6061b88e099fc7f8ad307ab.acm-validations.aws."
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
          "Name": "_74fbcdbf0b779e405cdb632b3678e67a.api.example.com.",
          "Type": "CNAME",
          "Value": "_cf519108b1c5fef726c5823fd8a53e80.acm-validations.aws."
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