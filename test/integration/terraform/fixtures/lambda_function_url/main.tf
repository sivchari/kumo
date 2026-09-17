data "aws_iam_policy_document" "assume" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "demo" {
  name               = "tf-lambda-function-url-demo"
  assume_role_policy = data.aws_iam_policy_document.assume.json
}

data "archive_file" "demo" {
  type        = "zip"
  output_path = "${path.module}/handler.zip"

  source {
    content  = "exports.handler = async () => ({ statusCode: 200 });"
    filename = "index.js"
  }
}

resource "aws_lambda_function" "demo" {
  function_name    = "tf-lambda-function-url-demo"
  role             = aws_iam_role.demo.arn
  runtime          = "nodejs20.x"
  handler          = "index.handler"
  filename         = data.archive_file.demo.output_path
  source_code_hash = data.archive_file.demo.output_base64sha256
}

resource "aws_lambda_function_url" "demo" {
  function_name      = aws_lambda_function.demo.function_name
  authorization_type = "NONE"

  cors {
    allow_credentials = false
    allow_origins     = ["*"]
    allow_methods     = ["GET", "POST"]
    allow_headers     = ["content-type"]
    expose_headers    = ["x-request-id"]
    max_age           = 3600
  }
}

resource "aws_lambda_permission" "url" {
  statement_id           = "AllowPublicFunctionUrl"
  action                 = "lambda:InvokeFunctionUrl"
  function_name          = aws_lambda_function.demo.function_name
  principal              = "*"
  function_url_auth_type = "NONE"
}

# Read the URL back and assert the shape AWS returns.
data "aws_lambda_function_url" "demo" {
  function_name = aws_lambda_function_url.demo.function_name

  lifecycle {
    postcondition {
      condition     = startswith(self.function_url, "https://") && strcontains(self.function_url, ".lambda-url.")
      error_message = "function_url must be an https lambda-url endpoint"
    }

    postcondition {
      condition     = length(self.url_id) == 32
      error_message = "url_id must be the 32-character function URL id"
    }

    postcondition {
      condition     = self.authorization_type == "NONE"
      error_message = "authorization_type must read back as NONE"
    }
  }
}
