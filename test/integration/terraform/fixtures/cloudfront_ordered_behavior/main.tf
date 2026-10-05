locals {
  origin_id = "tf-cloudfront-ordered-behavior-origin"

  # Managed CachingDisabled cache policy.
  caching_disabled_policy_id = "4135ea2d-6df8-44a3-9df3-4b5a84be39ad"
}

resource "aws_cloudfront_distribution" "ordered" {
  comment             = "tf-cloudfront-ordered-behavior"
  enabled             = true
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

  # Modern behavior: a managed cache policy instead of forwarded_values.
  ordered_cache_behavior {
    path_pattern           = "/api/*"
    target_origin_id       = local.origin_id
    viewer_protocol_policy = "redirect-to-https"
    allowed_methods        = ["GET", "HEAD", "OPTIONS"]
    cached_methods         = ["GET", "HEAD"]
    cache_policy_id        = local.caching_disabled_policy_id
    compress               = true
  }

  # Legacy behavior: inline forwarded_values and explicit TTLs.
  ordered_cache_behavior {
    path_pattern           = "/static/*.jpg"
    target_origin_id       = local.origin_id
    viewer_protocol_policy = "https-only"
    allowed_methods        = ["GET", "HEAD"]
    cached_methods         = ["GET", "HEAD"]
    min_ttl                = 0
    default_ttl            = 3600
    max_ttl                = 86400

    forwarded_values {
      query_string = true
      headers      = ["Origin"]

      cookies {
        forward = "none"
      }
    }
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
      condition     = length(self.ordered_cache_behavior) == 2
      error_message = "both ordered cache behaviors must read back, got ${length(self.ordered_cache_behavior)}"
    }

    postcondition {
      condition     = self.ordered_cache_behavior[0].path_pattern == "/api/*" && self.ordered_cache_behavior[1].path_pattern == "/static/*.jpg"
      error_message = "ordered cache behaviors must keep their list order and path patterns"
    }

    postcondition {
      condition = (
        self.ordered_cache_behavior[0].cache_policy_id == local.caching_disabled_policy_id &&
        length(self.ordered_cache_behavior[0].forwarded_values) == 0 &&
        self.ordered_cache_behavior[0].viewer_protocol_policy == "redirect-to-https" &&
        self.ordered_cache_behavior[0].compress
      )
      error_message = "cache policy behavior must read back its cache_policy_id without forwarded_values"
    }

    postcondition {
      condition = (
        (self.ordered_cache_behavior[1].cache_policy_id == null || self.ordered_cache_behavior[1].cache_policy_id == "") &&
        length(self.ordered_cache_behavior[1].forwarded_values) == 1 &&
        self.ordered_cache_behavior[1].forwarded_values[0].query_string &&
        contains(self.ordered_cache_behavior[1].forwarded_values[0].headers, "Origin") &&
        self.ordered_cache_behavior[1].forwarded_values[0].cookies[0].forward == "none"
      )
      error_message = "legacy behavior must read back its forwarded_values and no cache_policy_id"
    }

    postcondition {
      condition = (
        self.ordered_cache_behavior[1].default_ttl == 3600 &&
        self.ordered_cache_behavior[1].max_ttl == 86400 &&
        self.ordered_cache_behavior[1].viewer_protocol_policy == "https-only"
      )
      error_message = "legacy behavior must read back its TTLs and viewer protocol policy"
    }
  }
}
