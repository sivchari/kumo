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
  name               = "tf-s3-lambda-notification-demo"
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
  function_name    = "tf-s3-lambda-notification-demo"
  role             = aws_iam_role.demo.arn
  runtime          = "nodejs20.x"
  handler          = "index.handler"
  filename         = data.archive_file.demo.output_path
  source_code_hash = data.archive_file.demo.output_base64sha256
}

resource "aws_s3_bucket" "demo" {
  bucket = "tf-s3-lambda-notification-demo"
}

resource "aws_lambda_permission" "demo" {
  statement_id  = "AllowS3Invoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.demo.function_name
  principal     = "s3.amazonaws.com"
  source_arn    = aws_s3_bucket.demo.arn
}

resource "aws_s3_bucket_notification" "demo" {
  bucket = aws_s3_bucket.demo.id

  lambda_function {
    lambda_function_arn = aws_lambda_function.demo.arn
    events              = ["s3:ObjectCreated:*"]
    filter_prefix       = "uploads/"
    filter_suffix       = ".jpg"
  }

  depends_on = [aws_lambda_permission.demo]
}
