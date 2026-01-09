variable "aws_region" {
  type    = string
  default = "us-east-1"
}

variable "instance_type" {
  type    = string
  default = "t3.micro"
}

variable "allowed_cidr" {
  description = "Who can access port 3000/22 (use your IP/32 in real life)"
  type        = string
  default     = "0.0.0.0/0"
}

variable "key_name" {
  description = "Optional: existing EC2 key pair name for SSH"
  type        = string
  default     = null
}
