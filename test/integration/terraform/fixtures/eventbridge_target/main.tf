resource "aws_sqs_queue" "target" {
  name = "tf-eventbridge-target-queue"
}

resource "aws_cloudwatch_event_rule" "demo" {
  name          = "tf-eventbridge-target-rule"
  event_pattern = "{\"source\":[\"kumo.terraform\"]}"

  tags = {
    env = "terraform"
  }
}

resource "aws_cloudwatch_event_target" "demo" {
  rule      = aws_cloudwatch_event_rule.demo.name
  target_id = "tf-eventbridge-target-id"
  arn       = aws_sqs_queue.target.arn
}
