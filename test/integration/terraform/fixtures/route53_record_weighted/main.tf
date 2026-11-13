resource "aws_route53_zone" "main" {
  name = "tf-route53-record-weighted.example.com"
}

# Two weighted record sets share a name and type and differ only in
# set_identifier.
resource "aws_route53_record" "blue" {
  zone_id        = aws_route53_zone.main.zone_id
  name           = "www"
  type           = "A"
  ttl            = 60
  records        = ["192.0.2.1"]
  set_identifier = "blue"

  weighted_routing_policy {
    weight = 10
  }
}

resource "aws_route53_record" "green" {
  zone_id        = aws_route53_zone.main.zone_id
  name           = "www"
  type           = "A"
  ttl            = 60
  records        = ["192.0.2.2"]
  set_identifier = "green"

  weighted_routing_policy {
    weight = 90
  }
}
