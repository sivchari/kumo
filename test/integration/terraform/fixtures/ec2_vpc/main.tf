resource "aws_vpc" "demo" {
  cidr_block = "10.42.0.0/16"

  tags = {
    Name = "tf-ec2-vpc-demo"
  }
}
