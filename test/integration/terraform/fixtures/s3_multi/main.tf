resource "aws_s3_bucket" "logs" {
  bucket = "tf-s3-multi-logs"
}

resource "aws_s3_bucket" "data" {
  bucket = "tf-s3-multi-data"
}

resource "aws_s3_object" "config" {
  bucket       = aws_s3_bucket.data.id
  key          = "config.json"
  content      = "{\"k\":\"v\"}"
  content_type = "application/json"
}

resource "aws_s3_object" "nested" {
  bucket  = aws_s3_bucket.data.id
  key     = "nested/file.txt"
  content = "nested content"
}
