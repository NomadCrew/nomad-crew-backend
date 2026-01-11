# AWS Infrastructure - NomadCrew Backend
# Cost-optimized EC2 instance with Graviton (ARM) processor
#
# Estimated costs (us-east-2):
#   t4g.small  (2 vCPU, 2 GB):  ~$12/month
#   t4g.medium (2 vCPU, 4 GB):  ~$24/month
#   + EBS 30GB gp3:             ~$2.40/month
#   + Elastic IP:               ~$3.60/month (if instance running, free)
#   Total:                      ~$15-30/month depending on instance size

terraform {
  required_version = ">= 1.5.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.region

  default_tags {
    tags = {
      Project     = "nomadcrew"
      Environment = "production"
      ManagedBy   = "terraform"
    }
  }
}

# -----------------------------------------------------------------------------
# Data Sources
# -----------------------------------------------------------------------------

# Get latest Ubuntu 22.04 ARM AMI
data "aws_ami" "ubuntu_arm" {
  most_recent = true
  owners      = ["099720109477"] # Canonical

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-arm64-server-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  filter {
    name   = "architecture"
    values = ["arm64"]
  }
}

# Get available AZs
data "aws_availability_zones" "available" {
  state = "available"
}

# -----------------------------------------------------------------------------
# VPC and Networking
# -----------------------------------------------------------------------------

resource "aws_vpc" "main" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = {
    Name = "nomadcrew-vpc"
  }
}

resource "aws_internet_gateway" "main" {
  vpc_id = aws_vpc.main.id

  tags = {
    Name = "nomadcrew-igw"
  }
}

resource "aws_subnet" "public" {
  vpc_id                  = aws_vpc.main.id
  cidr_block              = "10.0.1.0/24"
  availability_zone       = data.aws_availability_zones.available.names[0]
  map_public_ip_on_launch = true

  tags = {
    Name = "nomadcrew-public-subnet"
  }
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.main.id
  }

  tags = {
    Name = "nomadcrew-public-rt"
  }
}

resource "aws_route_table_association" "public" {
  subnet_id      = aws_subnet.public.id
  route_table_id = aws_route_table.public.id
}

# -----------------------------------------------------------------------------
# Security Group
# -----------------------------------------------------------------------------

resource "aws_security_group" "backend" {
  name        = "nomadcrew-backend-sg"
  description = "Security group for NomadCrew backend"
  vpc_id      = aws_vpc.main.id

  # SSH
  ingress {
    description = "SSH"
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = var.ssh_allowed_cidrs
  }

  # HTTP
  ingress {
    description = "HTTP"
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  # HTTPS
  ingress {
    description = "HTTPS"
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  # All outbound traffic
  egress {
    description = "All outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "nomadcrew-backend-sg"
  }
}

# -----------------------------------------------------------------------------
# SSH Key Pair
# -----------------------------------------------------------------------------

resource "aws_key_pair" "deployer" {
  key_name   = "nomadcrew-deployer"
  public_key = file(var.ssh_public_key_path)
}

# -----------------------------------------------------------------------------
# EC2 Instance
# -----------------------------------------------------------------------------

resource "aws_instance" "backend" {
  ami                    = data.aws_ami.ubuntu_arm.id
  instance_type          = var.instance_type
  key_name               = aws_key_pair.deployer.key_name
  vpc_security_group_ids = [aws_security_group.backend.id]
  subnet_id              = aws_subnet.public.id

  root_block_device {
    volume_size           = var.root_volume_size
    volume_type           = "gp3"
    iops                  = 3000
    throughput            = 125
    delete_on_termination = true
    encrypted             = true
  }

  # Enable detailed monitoring (additional cost but useful)
  monitoring = false

  # User data to set hostname and update system
  user_data = <<-EOF
    #!/bin/bash
    hostnamectl set-hostname nomadcrew-backend
    apt-get update
    apt-get upgrade -y
  EOF

  tags = {
    Name = "nomadcrew-backend"
  }

  lifecycle {
    ignore_changes = [ami] # Don't recreate on AMI updates
  }
}

# -----------------------------------------------------------------------------
# Elastic IP (stable public IP)
# -----------------------------------------------------------------------------

resource "aws_eip" "backend" {
  instance = aws_instance.backend.id
  domain   = "vpc"

  tags = {
    Name = "nomadcrew-backend-eip"
  }

  depends_on = [aws_internet_gateway.main]
}
