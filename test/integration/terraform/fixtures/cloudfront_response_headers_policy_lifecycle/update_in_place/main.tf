locals {
  origin_id = "tf-cloudfront-response-headers-policy-lifecycle-origin"
}

resource "aws_cloudfront_response_headers_policy" "custom" {
  name    = "tf-cloudfront-response-headers-policy-lifecycle"
  comment = "updated"

  cors_config {
    access_control_allow_credentials = true
    access_control_max_age_sec       = 0
    origin_override                  = false

    access_control_allow_headers {
      items = ["Content-Type", "Authorization"]
    }

    access_control_allow_methods {
      items = ["GET", "HEAD", "OPTIONS", "POST"]
    }

    access_control_allow_origins {
      items = ["https://www.example.com", "https://app.example.com"]
    }

    access_control_expose_headers {
      items = ["X-Request-Id"]
    }
  }

  custom_headers_config {
    items {
      header   = "X-Kumo-Fixture"
      override = true
      value    = "updated"
    }

    items {
      header   = "X-Kumo-Extra"
      override = false
      value    = "extra"
    }
  }

  remove_headers_config {
    items {
      header = "X-Powered-By"
    }
  }

  security_headers_config {
    content_security_policy {
      content_security_policy = "default-src 'self'"
      override                = true
    }

    content_type_options {
      override = false
    }

    frame_options {
      frame_option = "SAMEORIGIN"
      override     = false
    }

    referrer_policy {
      referrer_policy = "strict-origin-when-cross-origin"
      override        = true
    }

    strict_transport_security {
      access_control_max_age_sec = 31536000
      include_subdomains         = true
      preload                    = false
      override                   = true
    }

    xss_protection {
      mode_block = true
      protection = true
      override   = false
    }
  }

  server_timing_headers_config {
    enabled       = true
    sampling_rate = 12.5
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
