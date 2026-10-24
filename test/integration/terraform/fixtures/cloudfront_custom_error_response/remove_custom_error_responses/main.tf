locals {
  origin_id = "tf-cloudfront-custom-error-response-origin"

  # Managed CachingDisabled cache policy.
  caching_disabled_policy_id = "4135ea2d-6df8-44a3-9df3-4b5a84be39ad"
}

resource "aws_cloudfront_distribution" "custom_errors" {
  comment             = "tf-cloudfront-custom-error-response"
  enabled             = true
  wait_for_deployment = false

  origin {
    origin_id   = local.origin_id
    domain_name = "tf-cloudfront-custom-error-response-origin.s3.us-east-1.amazonaws.com"
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

  lifecycle {
    postcondition {
      condition     = length(self.custom_error_response) == 0
      error_message = "removed custom error responses must not read back, got ${length(self.custom_error_response)}"
    }
  }
}
