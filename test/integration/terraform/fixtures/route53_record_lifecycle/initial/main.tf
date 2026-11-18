resource "aws_route53_zone" "main" {
  name = "tf-route53-record-lifecycle.example.com"
}

resource "aws_route53_record" "www" {
  zone_id = aws_route53_zone.main.zone_id
  name    = "www"
  type    = "A"
  ttl     = 300
  records = ["192.0.2.1"]
}

output "www_fqdn" {
  value = aws_route53_record.www.fqdn
}
