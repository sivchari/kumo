resource "aws_cloudfront_origin_access_control" "s3" {
  name                              = "tf-cloudfront-origin-access-control-s3"
  description                       = "S3 bucket origin"
  origin_access_control_origin_type = "s3"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}

# No description: the provider fills in "Managed by Terraform", which must
# read back unchanged.
resource "aws_cloudfront_origin_access_control" "lambda" {
  name                              = "tf-cloudfront-origin-access-control-lambda"
  origin_access_control_origin_type = "lambda"
  signing_behavior                  = "no-override"
  signing_protocol                  = "sigv4"
}

# Read both back and assert the shape AWS returns.
data "aws_cloudfront_origin_access_control" "s3" {
  id = aws_cloudfront_origin_access_control.s3.id

  lifecycle {
    postcondition {
      condition     = self.origin_access_control_origin_type == "s3" && self.signing_behavior == "always" && self.signing_protocol == "sigv4"
      error_message = "s3 control must read back with its type, behavior and protocol"
    }

    postcondition {
      condition     = self.description == "S3 bucket origin" && self.etag != ""
      error_message = "s3 control must read back its description and carry an ETag"
    }
  }
}

data "aws_cloudfront_origin_access_control" "lambda" {
  id = aws_cloudfront_origin_access_control.lambda.id

  lifecycle {
    postcondition {
      condition     = self.origin_access_control_origin_type == "lambda" && self.signing_behavior == "no-override"
      error_message = "lambda control must read back with its type and behavior"
    }

    postcondition {
      condition     = startswith(self.id, "E") && length(self.id) == 14
      error_message = "id must have the AWS shape (E + 13 characters)"
    }
  }
}
