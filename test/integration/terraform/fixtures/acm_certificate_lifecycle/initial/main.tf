resource "aws_acm_certificate" "site" {
  domain_name               = "tf-acm-certificate-lifecycle.example.com"
  subject_alternative_names = [
    "*.tf-acm-certificate-lifecycle.example.com",
    "www.tf-acm-certificate-lifecycle.example.com",
  ]
  validation_method         = "DNS"
}

# Without validation_record_fqdns the resource only waits for the
# certificate to reach ISSUED.
resource "aws_acm_certificate_validation" "site" {
  certificate_arn = aws_acm_certificate.site.arn
}

output "certificate_status" {
  value = aws_acm_certificate.site.status
}

locals {
  validation_record_names = {
    for dvo in aws_acm_certificate.site.domain_validation_options : dvo.domain_name => dvo.resource_record_name
  }
}

# The wildcard SAN shares its base domain's validation record, so three
# names need only two records.
output "wildcard_shares_base_record" {
  value = tostring(
    local.validation_record_names["*.tf-acm-certificate-lifecycle.example.com"] ==
    local.validation_record_names["tf-acm-certificate-lifecycle.example.com"]
  )
}

output "validation_record_count" {
  value = tostring(length(distinct(values(local.validation_record_names))))
}
