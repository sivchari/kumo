{
  "Certificate": {
    "CertificateArn": "arn:aws:acm:us-east-1:000000000000:certificate/6421d74b-5678-4057-bb4d-1c820f57397d",
    "CertificateAuthorityArn": null,
    "CreatedAt": "2026-05-15T04:58:29.748Z",
    "DomainName": "example.com",
    "DomainValidationOptions": [
      {
        "DomainName": "api.example.com",
        "HttpRedirect": null,
        "ResourceRecord": {
          "Name": "_d930b28be6c5927595552b219965053e.api.example.com.",
          "Type": "CNAME",
          "Value": "_c9edd76ee4a0e2a74388032f3861cc50.ykybfrwcxw.acm-validations.aws."
        },
        "ValidationDomain": "api.example.com",
        "ValidationEmails": null,
        "ValidationMethod": "DNS",
        "ValidationStatus": "PENDING_VALIDATION"
      },
      {
        "DomainName": "www.example.com",
        "HttpRedirect": null,
        "ResourceRecord": {
          "Name": "_d930b28be6c5927595552b219965053e.www.example.com.",
          "Type": "CNAME",
          "Value": "_c9edd76ee4a0e2a74388032f3861cc50.ykybfrwcxw.acm-validations.aws."
        },
        "ValidationDomain": "www.example.com",
        "ValidationEmails": null,
        "ValidationMethod": "DNS",
        "ValidationStatus": "PENDING_VALIDATION"
      },
      {
        "DomainName": "example.com",
        "HttpRedirect": null,
        "ResourceRecord": {
          "Name": "_d930b28be6c5927595552b219965053e.example.com.",
          "Type": "CNAME",
          "Value": "_c9edd76ee4a0e2a74388032f3861cc50.ykybfrwcxw.acm-validations.aws."
        },
        "ValidationDomain": "example.com",
        "ValidationEmails": null,
        "ValidationMethod": "DNS",
        "ValidationStatus": "PENDING_VALIDATION"
      }
    ],
    "ExtendedKeyUsages": [],
    "FailureReason": "",
    "ImportedAt": null,
    "InUseBy": [],
    "IssuedAt": null,
    "Issuer": "Amazon",
    "KeyAlgorithm": "RSA-2048",
    "KeyUsages": [],
    "ManagedBy": "",
    "NotAfter": null,
    "NotBefore": null,
    "Options": {
      "CertificateTransparencyLoggingPreference": "ENABLED",
      "Export": ""
    },
    "RenewalEligibility": "INELIGIBLE",
    "RenewalSummary": null,
    "RevocationReason": "",
    "RevokedAt": null,
    "Serial": null,
    "SignatureAlgorithm": "SHA512WITHRSA",
    "Status": "PENDING_VALIDATION",
    "Subject": "CN=example.com",
    "SubjectAlternativeNames": [
      "api.example.com",
      "www.example.com",
      "example.com"
    ],
    "Type": "AMAZON_ISSUED"
  },
  "ResultMetadata": {}
}