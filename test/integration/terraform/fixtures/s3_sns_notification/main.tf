resource "aws_sns_topic" "demo" {
  name = "tf-s3-sns-notification-demo"
}

data "aws_iam_policy_document" "demo" {
  statement {
    actions   = ["SNS:Publish"]
    resources = [aws_sns_topic.demo.arn]

    principals {
      type        = "Service"
      identifiers = ["s3.amazonaws.com"]
    }
  }
}

resource "aws_sns_topic_policy" "demo" {
  arn    = aws_sns_topic.demo.arn
  policy = data.aws_iam_policy_document.demo.json
}

resource "aws_s3_bucket" "demo" {
  bucket = "tf-s3-sns-notification-demo"
}

resource "aws_s3_bucket_notification" "demo" {
  bucket = aws_s3_bucket.demo.id

  topic {
    topic_arn     = aws_sns_topic.demo.arn
    events        = ["s3:ObjectCreated:*"]
    filter_prefix = "uploads/"
    filter_suffix = ".jpg"
  }

  depends_on = [aws_sns_topic_policy.demo]
}
