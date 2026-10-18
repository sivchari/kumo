resource "aws_acm_certificate" "site" {
  domain_name               = "tf-acm-certificate-lifecycle.example.com"
  subject_alternative_names = ["www.tf-acm-certificate-lifecycle.example.com"]
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

output "validation_record_names" {
  value = join(",", sort([for dvo in aws_acm_certificate.site.domain_validation_options : dvo.resource_record_name]))
}
