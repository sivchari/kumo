resource "aws_acm_certificate" "site" {
  domain_name               = "tf-acm-certificate-lifecycle.example.com"
  subject_alternative_names = [
    "*.tf-acm-certificate-lifecycle.example.com",
    "www.tf-acm-certificate-lifecycle.example.com",
  ]
  validation_method         = "DNS"

  tags = {
    env  = "dev"
    team = "web"
  }
}

resource "aws_acm_certificate_validation" "site" {
  certificate_arn = aws_acm_certificate.site.arn
}
