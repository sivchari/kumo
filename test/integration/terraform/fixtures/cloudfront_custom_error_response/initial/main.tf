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

  # A single-page app's fallback: missing paths serve index.html with 200.
  custom_error_response {
    error_code            = 404
    response_code         = 200
    response_page_path    = "/index.html"
    error_caching_min_ttl = 10
  }

  # Error caching only, without a custom error page.
  custom_error_response {
    error_code            = 503
    error_caching_min_ttl = 5
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
      condition     = length(self.custom_error_response) == 2
      error_message = "both custom error responses must read back, got ${length(self.custom_error_response)}"
    }

    postcondition {
      condition = alltrue([
        for r in self.custom_error_response :
        r.error_code == 404 ? (
          r.response_code == 200 && r.response_page_path == "/index.html" && r.error_caching_min_ttl == 10
          ) : (
          r.error_code == 503 && r.response_page_path == "" && r.error_caching_min_ttl == 5
        )
      ])
      error_message = "custom error responses must read back with their response code, page path and error caching TTL"
    }
  }
}
