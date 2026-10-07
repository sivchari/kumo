locals {
  origin_id = "tf-cloudfront-cache-policy-lifecycle-origin"
}

resource "aws_cloudfront_cache_policy" "custom" {
  name        = "tf-cloudfront-cache-policy-lifecycle"
  comment     = "initial"
  default_ttl = 3600
  max_ttl     = 86400
  min_ttl     = 0

  parameters_in_cache_key_and_forwarded_to_origin {
    enable_accept_encoding_gzip   = true
    enable_accept_encoding_brotli = false

    cookies_config {
      cookie_behavior = "none"
    }

    headers_config {
      header_behavior = "whitelist"

      headers {
        items = ["Origin"]
      }
    }

    query_strings_config {
      query_string_behavior = "whitelist"

      query_strings {
        items = ["v"]
      }
    }
  }
}

resource "aws_cloudfront_distribution" "web" {
  comment             = "tf-cloudfront-cache-policy-lifecycle"
  enabled             = true
  wait_for_deployment = false

  origin {
    origin_id   = local.origin_id
    domain_name = "origin.example.com"

    custom_origin_config {
      http_port              = 80
      https_port             = 443
      origin_protocol_policy = "https-only"
      origin_ssl_protocols   = ["TLSv1.2"]
    }
  }

  default_cache_behavior {
    target_origin_id       = local.origin_id
    viewer_protocol_policy = "redirect-to-https"
    allowed_methods        = ["GET", "HEAD"]
    cached_methods         = ["GET", "HEAD"]
    cache_policy_id        = aws_cloudfront_cache_policy.custom.id
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
