resource "aws_s3_bucket" "tagged" {
  bucket = "tf-s3-bucket-tags"

  tags = {
    Environment = "dev"
    Owner       = "team-a"
  }
}

output "bucket_id" {
  value = aws_s3_bucket.tagged.id
}
