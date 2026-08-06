{
  "Certificate": {
    "CertificateArn": "arn:aws:acm:us-east-1:000000000000:certificate/75a84988-284a-49ed-b91b-c86dd7ce4ae0",
    "CertificateAuthorityArn": null,
    "CreatedAt": "2026-08-06T07:46:54.137Z",
    "DomainName": "cli-test.example.com",
    "DomainValidationOptions": [
      {
        "DomainName": "cli-test.example.com",
        "HttpRedirect": null,
        "ResourceRecord": {
          "Name": "_acme-challenge.cli-test.example.com",
          "Type": "CNAME",
          "Value": "_75a84988.acm-validations.aws."
        },
        "ValidationDomain": "cli-test.example.com",
        "ValidationEmails": null,
        "ValidationMethod": "DNS",
        "ValidationStatus": "PENDING_VALIDATION"
      }
    ],
    "ExtendedKeyUsages": null,
    "FailureReason": "",
    "ImportedAt": null,
    "InUseBy": null,
    "IssuedAt": null,
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
    "Serial": "4a4a3b7eefa5a788de7c47ab83a4da3f",
    "SignatureAlgorithm": null,
    "Status": "PENDING_VALIDATION",
    "Subject": "CN=cli-test.example.com",
    "SubjectAlternativeNames": null,
    "Type": "AMAZON_ISSUED"
  },
  "ResultMetadata": {}
}