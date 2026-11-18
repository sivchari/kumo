resource "aws_acm_certificate" "site" {
  domain_name               = "tf-acm-certificate-tags.example.com"
  subject_alternative_names = ["www.tf-acm-certificate-tags.example.com"]
  validation_method         = "DNS"

  # Changes env, drops team and adds owner, so the provider calls both
  # AddTagsToCertificate and RemoveTagsFromCertificate.
  tags = {
    env   = "prod"
    owner = "alice"
  }
}

output "tags" {
  value = jsonencode(aws_acm_certificate.site.tags_all)
}
