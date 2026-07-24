resource "aws_secretsmanager_secret" "demo" {
  name                    = "tf-secretsmanager-secret-demo"
  description             = "managed by terraform integration test"
  recovery_window_in_days = 0

  tags = {
    env = "terraform"
  }
}

resource "aws_secretsmanager_secret_version" "demo" {
  secret_id     = aws_secretsmanager_secret.demo.id
  secret_string = "{\"hello\":\"terraform\"}"
}
