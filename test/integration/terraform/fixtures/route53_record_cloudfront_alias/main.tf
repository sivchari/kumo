resource "aws_route53_zone" "main" {
  name = "tf-route53-record-cloudfront-alias.example.com"
}

locals {
  origin_id = "tf-route53-record-cloudfront-alias-origin"

  # Managed CachingDisabled cache policy.
  caching_disabled_policy_id = "4135ea2d-6df8-44a3-9df3-4b5a84be39ad"
}

resource "aws_cloudfront_distribution" "cdn" {
  comment             = "tf-route53-record-cloudfront-alias"
  enabled             = true
  is_ipv6_enabled     = true
  wait_for_deployment = false

  origin {
    origin_id   = local.origin_id
    domain_name = "origin.example.com"

    custom_origin_config {
      http_port              = 80
      https_port             = 443
      origin_protocol_policy = "http-only"
      origin_ssl_protocols   = ["TLSv1.2"]
    }
  }

  default_cache_behavior {
    target_origin_id       = local.origin_id
    viewer_protocol_policy = "allow-all"
    allowed_methods        = ["GET", "HEAD"]
    cached_methods         = ["GET", "HEAD"]
    cache_policy_id        = local.caching_disabled_policy_id
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  viewer_certificate {
    cloudfront_default_certificate = true
  }
}

# One alias record is named fully qualified with the trailing dot and the
# other relative to the zone; both must read back after create.
resource "aws_route53_record" "cdn_a" {
  zone_id = aws_route53_zone.main.zone_id
  name    = "cdn.tf-route53-record-cloudfront-alias.example.com."
  type    = "A"

  alias {
    name                   = aws_cloudfront_distribution.cdn.domain_name
    zone_id                = aws_cloudfront_distribution.cdn.hosted_zone_id
    evaluate_target_health = false
  }
}

resource "aws_route53_record" "cdn_aaaa" {
  zone_id = aws_route53_zone.main.zone_id
  name    = "cdn"
  type    = "AAAA"

  alias {
    name                   = aws_cloudfront_distribution.cdn.domain_name
    zone_id                = aws_cloudfront_distribution.cdn.hosted_zone_id
    evaluate_target_health = false
  }
}
