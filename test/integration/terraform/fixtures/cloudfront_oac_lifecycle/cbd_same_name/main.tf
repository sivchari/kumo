locals {
  origin_id = "tf-cloudfront-oac-lifecycle-origin"

  # Managed CachingDisabled cache policy.
  caching_disabled_policy_id = "4135ea2d-6df8-44a3-9df3-4b5a84be39ad"
}

resource "aws_cloudfront_origin_access_control" "s3" {
  name                              = "tf-cloudfront-oac-lifecycle-s3"
  description                       = "updated"
  origin_access_control_origin_type = "s3"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_cloudfront_distribution" "s3" {
  comment             = "tf-cloudfront-oac-lifecycle"
  enabled             = true
  wait_for_deployment = false

  origin {
    origin_id                = local.origin_id
    domain_name              = "tf-cloudfront-oac-lifecycle.s3.us-east-1.amazonaws.com"
    origin_access_control_id = aws_cloudfront_origin_access_control.s3.id
  }

  default_cache_behavior {
    target_origin_id       = local.origin_id
    viewer_protocol_policy = "redirect-to-https"
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
