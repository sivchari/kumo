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
  name               = "tf-lambda-function-demo"
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
  function_name    = "tf-lambda-function-demo"
  role             = aws_iam_role.demo.arn
  runtime          = "nodejs20.x"
  handler          = "index.handler"
  filename         = data.archive_file.demo.output_path
  source_code_hash = data.archive_file.demo.output_base64sha256

  environment {
    variables = {
      APP_ENV = "test"
    }
  }

  tags = {
    env = "terraform"
  }
}

output "function_name" {
  value = aws_lambda_function.demo.function_name
}
