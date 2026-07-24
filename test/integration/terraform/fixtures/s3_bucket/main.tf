resource "aws_s3_bucket" "demo" {
  bucket = "tf-s3-bucket-demo"
}

resource "aws_s3_object" "hello" {
  bucket  = aws_s3_bucket.demo.id
  key     = "hello.txt"
  content = "hello from terraform"
}

output "bucket_id" {
  value = aws_s3_bucket.demo.id
}
