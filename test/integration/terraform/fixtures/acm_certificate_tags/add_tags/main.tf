resource "aws_acm_certificate" "site" {
  domain_name               = "tf-acm-certificate-tags.example.com"
  subject_alternative_names = ["www.tf-acm-certificate-tags.example.com"]
  validation_method         = "DNS"

  tags = {
    env  = "dev"
    team = "web"
  }
}
