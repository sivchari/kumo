resource "aws_dynamodb_table" "demo" {
  name         = "tf-dynamodb-table-demo"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "pk"

  attribute {
    name = "pk"
    type = "S"
  }

  tags = {
    env = "terraform"
  }
}
