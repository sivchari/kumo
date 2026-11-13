resource "aws_acm_certificate" "site" {
  domain_name               = "tf-acm-certificate-lifecycle.example.com"
  subject_alternative_names = [
    "*.tf-acm-certificate-lifecycle.example.com",
    "www.tf-acm-certificate-lifecycle.example.com",
  ]
  validation_method         = "DNS"

  # Changes env, drops team and adds owner, so the provider calls both
  # AddTagsToCertificate and RemoveTagsFromCertificate.
  tags = {
    env   = "prod"
    owner = "alice"
  }
}

resource "aws_acm_certificate_validation" "site" {
  certificate_arn = aws_acm_certificate.site.arn
}

output "tags" {
  value = jsonencode(aws_acm_certificate.site.tags_all)
}
