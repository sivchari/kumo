locals {
  origin_id = "tf-cloudfront-response-headers-policy-lifecycle-origin"
}

resource "aws_cloudfront_response_headers_policy" "custom" {
  name    = "tf-cloudfront-response-headers-policy-lifecycle"
  comment = "initial"

  cors_config {
    access_control_allow_credentials = false
    access_control_max_age_sec       = 600
    origin_override                  = true

    access_control_allow_headers {
      items = ["Content-Type"]
    }

    access_control_allow_methods {
      items = ["GET", "HEAD", "OPTIONS"]
    }

    access_control_allow_origins {
      items = ["https://www.example.com"]
    }
  }

  custom_headers_config {
    items {
      header   = "X-Kumo-Fixture"
      override = false
      value    = "initial"
    }
  }

  security_headers_config {
    frame_options {
      frame_option = "DENY"
      override     = true
    }
  }
}

resource "aws_cloudfront_distribution" "web" {
  comment             = "tf-cloudfront-response-headers-policy-lifecycle"
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
    target_origin_id           = local.origin_id
    viewer_protocol_policy     = "redirect-to-https"
    allowed_methods            = ["GET", "HEAD"]
    cached_methods             = ["GET", "HEAD"]
    cache_policy_id            = "658327ea-f89d-4fab-a63d-7e88639e58f6"
    response_headers_policy_id = aws_cloudfront_response_headers_policy.custom.id
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
