resource "aws_sns_topic" "demo" {
  name = "tf-sns-topic-demo"

  tags = {
    env = "terraform"
  }
}

# Proves the postcondition pattern: read the topic back through a data
# source and fail the plan hard if the tag we set is missing.
data "aws_sns_topic" "demo" {
  name = aws_sns_topic.demo.name

  lifecycle {
    postcondition {
      condition     = self.tags["env"] == "terraform"
      error_message = "expected tag env=terraform on ${self.arn}, got ${jsonencode(self.tags)}"
    }
  }
}
