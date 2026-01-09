data "aws_availability_zones" "available" {}

# Latest Amazon Linux 2023 AMI (x86_64)
data "aws_ami" "al2023" {
  most_recent = true
  owners      = ["amazon"]

  filter {
    name   = "name"
    values = ["al2023-ami-*-x86_64"]
  }
}

resource "aws_vpc" "this" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = { Name = "tf-vpc" }
}

resource "aws_internet_gateway" "this" {
  vpc_id = aws_vpc.this.id
  tags   = { Name = "tf-igw" }
}

resource "aws_subnet" "public" {
  vpc_id                  = aws_vpc.this.id
  cidr_block              = "10.0.1.0/24"
  availability_zone       = data.aws_availability_zones.available.names[0]
  map_public_ip_on_launch = true

  tags = { Name = "tf-public-subnet" }
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.this.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.this.id
  }

  tags = { Name = "tf-public-rt" }
}

resource "aws_route_table_association" "public" {
  subnet_id      = aws_subnet.public.id
  route_table_id = aws_route_table.public.id
}

resource "aws_security_group" "web3000" {
  name        = "tf-sg-web3000"
  description = "Allow HTTP on 3000 and SSH"
  vpc_id      = aws_vpc.this.id

  ingress {
    description = "HTTP 3000"
    from_port   = 3000
    to_port     = 3000
    protocol    = "tcp"
    cidr_blocks = [var.allowed_cidr]
  }

  ingress {
    description = "SSH"
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = [var.allowed_cidr]
  }

  egress {
    description = "All outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = { Name = "tf-sg-web3000" }
}

resource "aws_instance" "web" {
  ami                    = data.aws_ami.al2023.id
  instance_type          = var.instance_type
  subnet_id              = aws_subnet.public.id
  vpc_security_group_ids = [aws_security_group.web3000.id]
  associate_public_ip_address = true

  # Optional SSH key
  key_name = var.key_name

  user_data = <<-EOF
              #!/bin/bash
              set -euxo pipefail

              # Simple HTTP server on port 3000 using Python
              cat >/home/ec2-user/index.html <<'HTML'
              <html>
                <head><title>EC2 on :3000</title></head>
                <body style="font-family: sans-serif;">
                  <h1>It works ✅</h1>
                  <p>Served from $(hostname) on port 3000.</p>
                </body>
              </html>
              HTML

              # systemd service
              cat >/etc/systemd/system/http3000.service <<'SERVICE'
              [Unit]
              Description=Simple HTTP server on port 3000
              After=network.target

              [Service]
              Type=simple
              User=ec2-user
              WorkingDirectory=/home/ec2-user
              ExecStart=/usr/bin/python3 -m http.server 3000 --bind 0.0.0.0
              Restart=always

              [Install]
              WantedBy=multi-user.target
              SERVICE

              systemctl daemon-reload
              systemctl enable --now http3000.service
              EOF

  tags = { Name = "tf-web-3000" }
}
