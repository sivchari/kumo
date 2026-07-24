resource "aws_sqs_queue" "demo" {
  name                       = "tf-sqs-queue-demo"
  visibility_timeout_seconds = 45

  tags = {
    env = "terraform"
  }
}
