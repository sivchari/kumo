resource "aws_cloudwatch_log_group" "demo" {
  name              = "tf-cloudwatchlogs-group-demo"
  retention_in_days = 7
}

resource "aws_cloudwatch_log_stream" "main" {
  name           = "tf-cloudwatchlogs-group-main"
  log_group_name = aws_cloudwatch_log_group.demo.name
}
