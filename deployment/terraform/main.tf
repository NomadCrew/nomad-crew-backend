provider "aws" {
  region = var.aws_region
}

# Configure Terraform backend for state management
terraform {
  backend "s3" {
    bucket         = "nomadcrew-terraform-state"
    key            = "nomadcrew/terraform.tfstate"
    region         = "us-east-1"
    encrypt        = true
    dynamodb_table = "nomadcrew-terraform-locks"
  }

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

# VPC Configuration
module "vpc" {
  source = "terraform-aws-modules/vpc/aws"
  version = "5.0.0"

  name = "${var.project_name}-vpc"
  cidr = var.vpc_cidr

  azs             = var.availability_zones
  private_subnets = var.private_subnet_cidrs
  public_subnets  = var.public_subnet_cidrs

  enable_nat_gateway = true
  single_nat_gateway = true  # Cost optimization: use only one NAT Gateway
  enable_vpn_gateway = false

  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = var.common_tags
}

# Security Groups
resource "aws_security_group" "alb_sg" {
  name        = "${var.project_name}-alb-sg"
  description = "Security group for ALB"
  vpc_id      = module.vpc.vpc_id

  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = var.common_tags
}

resource "aws_security_group" "app_sg" {
  name        = "${var.project_name}-app-sg"
  description = "Security group for application servers"
  vpc_id      = module.vpc.vpc_id

  ingress {
    from_port       = var.app_port
    to_port         = var.app_port
    protocol        = "tcp"
    security_groups = [aws_security_group.alb_sg.id]
  }

  ingress {
    from_port       = 22
    to_port         = 22
    protocol        = "tcp"
    cidr_blocks     = var.ssh_allowed_cidrs
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = var.common_tags
}

resource "aws_security_group" "db_sg" {
  name        = "${var.project_name}-db-sg"
  description = "Security group for RDS"
  vpc_id      = module.vpc.vpc_id

  ingress {
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_security_group.app_sg.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = var.common_tags
}

# RDS PostgreSQL (Free Tier)
resource "aws_db_subnet_group" "default" {
  name       = "${var.project_name}-db-subnet-group"
  subnet_ids = module.vpc.private_subnets

  tags = var.common_tags
}

resource "aws_db_instance" "postgres" {
  identifier             = "${var.project_name}-db"
  engine                 = "postgres"
  engine_version         = "14.10"  # Latest version eligible for free tier
  instance_class         = "db.t3.micro"  # Free tier eligible
  allocated_storage      = 20
  max_allocated_storage  = 100
  storage_type           = "gp2"
  username               = var.db_username
  password               = var.db_password
  db_name                = var.db_name
  parameter_group_name   = "default.postgres14"
  vpc_security_group_ids = [aws_security_group.db_sg.id]
  db_subnet_group_name   = aws_db_subnet_group.default.name
  skip_final_snapshot    = true
  backup_retention_period = 7
  backup_window          = "03:00-04:00"
  maintenance_window     = "Mon:04:00-Mon:05:00"
  multi_az               = false  # Free tier is single AZ only
  publicly_accessible    = false
  storage_encrypted      = true

  tags = var.common_tags
}

# Application Load Balancer
resource "aws_lb" "app_lb" {
  name               = "${var.project_name}-alb"
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb_sg.id]
  subnets            = module.vpc.public_subnets

  enable_deletion_protection = false

  tags = var.common_tags
}

resource "aws_lb_target_group" "app_tg" {
  name     = "${var.project_name}-tg"
  port     = var.app_port
  protocol = "HTTP"
  vpc_id   = module.vpc.vpc_id

  health_check {
    enabled             = true
    interval            = 30
    path                = "/health/liveness"
    port                = "traffic-port"
    healthy_threshold   = 3
    unhealthy_threshold = 3
    timeout             = 5
    protocol            = "HTTP"
    matcher             = "200"
  }

  tags = var.common_tags
}

resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.app_lb.arn
  port              = "80"
  protocol          = "HTTP"

  default_action {
    type = "redirect"

    redirect {
      port        = "443"
      protocol    = "HTTPS"
      status_code = "HTTP_301"
    }
  }
}

resource "aws_lb_listener" "https" {
  load_balancer_arn = aws_lb.app_lb.arn
  port              = "443"
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-2016-08"
  certificate_arn   = var.certificate_arn

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.app_tg.arn
  }
}

# Create ECR Repository if it doesn't exist
resource "aws_ecr_repository" "app" {
  name                 = var.project_name
  image_tag_mutability = "MUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }

  tags = var.common_tags
}

# ECR Repository Policy
resource "aws_ecr_repository_policy" "app_policy" {
  repository = aws_ecr_repository.app.name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "AllowPull"
        Effect = "Allow"
        Principal = {
          AWS = "arn:aws:iam::${data.aws_caller_identity.current.account_id}:root"
        }
        Action = [
          "ecr:GetDownloadUrlForLayer",
          "ecr:BatchGetImage",
          "ecr:BatchCheckLayerAvailability",
          "ecr:PullImage"
        ]
      }
    ]
  })
}

# Launch Template for EC2 instances
resource "aws_launch_template" "app" {
  name_prefix   = "${var.project_name}-lt-"
  image_id      = var.ami_id
  instance_type = "t2.micro"  # Free tier eligible
  key_name      = var.key_name

  vpc_security_group_ids = [aws_security_group.app_sg.id]

  iam_instance_profile {
    name = aws_iam_instance_profile.app_profile.name
  }

  block_device_mappings {
    device_name = "/dev/sda1"

    ebs {
      volume_size           = 8
      volume_type           = "gp2"
      delete_on_termination = true
    }
  }

  user_data = base64encode(templatefile("${path.module}/user_data.sh", {
    db_host           = aws_db_instance.postgres.address
    db_port           = aws_db_instance.postgres.port
    db_name           = aws_db_instance.postgres.db_name
    db_user           = aws_db_instance.postgres.username
    db_password       = aws_db_instance.postgres.password
    app_port          = var.app_port
    aws_region        = var.aws_region
    environment       = var.environment
    redis_password    = "redispass"  # Consider using a more secure password in production
    ecr_repository_uri = var.ecr_repository_uri != null ? var.ecr_repository_uri : aws_ecr_repository.app.repository_url
    image_tag         = var.image_tag
  }))

  tag_specifications {
    resource_type = "instance"
    tags = merge(
      var.common_tags,
      {
        Name = "${var.project_name}-app"
      }
    )
  }

  lifecycle {
    create_before_destroy = true
  }
}

# Auto Scaling Group
resource "aws_autoscaling_group" "app" {
  name                = "${var.project_name}-asg"
  vpc_zone_identifier = module.vpc.private_subnets
  min_size            = 1
  max_size            = 2
  desired_capacity    = 1

  launch_template {
    id      = aws_launch_template.app.id
    version = "$Latest"
  }

  target_group_arns = [aws_lb_target_group.app_tg.arn]

  health_check_type         = "ELB"
  health_check_grace_period = 300

  termination_policies = ["OldestInstance"]

  tag {
    key                 = "Name"
    value               = "${var.project_name}-app"
    propagate_at_launch = true
  }

  dynamic "tag" {
    for_each = var.common_tags

    content {
      key                 = tag.key
      value               = tag.value
      propagate_at_launch = true
    }
  }

  lifecycle {
    create_before_destroy = true
  }
}

# Auto Scaling Policies
resource "aws_autoscaling_policy" "scale_up" {
  name                   = "${var.project_name}-scale-up"
  scaling_adjustment     = 1
  adjustment_type        = "ChangeInCapacity"
  cooldown               = 300
  autoscaling_group_name = aws_autoscaling_group.app.name
}

resource "aws_autoscaling_policy" "scale_down" {
  name                   = "${var.project_name}-scale-down"
  scaling_adjustment     = -1
  adjustment_type        = "ChangeInCapacity"
  cooldown               = 300
  autoscaling_group_name = aws_autoscaling_group.app.name
}

# CloudWatch Alarms for Auto Scaling
resource "aws_cloudwatch_metric_alarm" "high_cpu" {
  alarm_name          = "${var.project_name}-high-cpu"
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = "2"
  metric_name         = "CPUUtilization"
  namespace           = "AWS/EC2"
  period              = "120"
  statistic           = "Average"
  threshold           = "80"
  alarm_description   = "This metric monitors high CPU utilization"
  alarm_actions       = [aws_autoscaling_policy.scale_up.arn]

  dimensions = {
    AutoScalingGroupName = aws_autoscaling_group.app.name
  }
}

resource "aws_cloudwatch_metric_alarm" "low_cpu" {
  alarm_name          = "${var.project_name}-low-cpu"
  comparison_operator = "LessThanOrEqualToThreshold"
  evaluation_periods  = "2"
  metric_name         = "CPUUtilization"
  namespace           = "AWS/EC2"
  period              = "120"
  statistic           = "Average"
  threshold           = "20"
  alarm_description   = "This metric monitors low CPU utilization"
  alarm_actions       = [aws_autoscaling_policy.scale_down.arn]

  dimensions = {
    AutoScalingGroupName = aws_autoscaling_group.app.name
  }
}

# S3 Bucket for application assets and backups
resource "aws_s3_bucket" "app_bucket" {
  bucket = "${var.project_name}-assets-${var.environment}"

  tags = var.common_tags
}

resource "aws_s3_bucket_ownership_controls" "app_bucket" {
  bucket = aws_s3_bucket.app_bucket.id
  rule {
    object_ownership = "BucketOwnerPreferred"
  }
}

resource "aws_s3_bucket_acl" "app_bucket" {
  depends_on = [aws_s3_bucket_ownership_controls.app_bucket]
  bucket = aws_s3_bucket.app_bucket.id
  acl    = "private"
}

resource "aws_s3_bucket_versioning" "app_bucket" {
  bucket = aws_s3_bucket.app_bucket.id
  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "app_bucket" {
  bucket = aws_s3_bucket.app_bucket.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

# IAM Role for EC2 instances
resource "aws_iam_role" "app_role" {
  name = "${var.project_name}-app-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "ec2.amazonaws.com"
        }
      }
    ]
  })

  tags = var.common_tags
}

resource "aws_iam_instance_profile" "app_profile" {
  name = "${var.project_name}-app-profile"
  role = aws_iam_role.app_role.name
}

resource "aws_iam_role_policy" "app_policy" {
  name = "${var.project_name}-app-policy"
  role = aws_iam_role.app_role.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = [
          "s3:GetObject",
          "s3:PutObject",
          "s3:ListBucket",
        ]
        Effect = "Allow"
        Resource = [
          aws_s3_bucket.app_bucket.arn,
          "${aws_s3_bucket.app_bucket.arn}/*",
        ]
      },
      {
        Action = [
          "secretsmanager:GetSecretValue",
        ]
        Effect = "Allow"
        Resource = [
          "arn:aws:secretsmanager:${var.aws_region}:${data.aws_caller_identity.current.account_id}:secret:${var.aws_secrets_path}/*",
        ]
      },
      {
        Action = [
          "cloudwatch:PutMetricData",
          "logs:CreateLogGroup",
          "logs:CreateLogStream",
          "logs:PutLogEvents",
        ]
        Effect = "Allow"
        Resource = "*"
      },
      {
        Action = [
          "ecr:GetDownloadUrlForLayer",
          "ecr:BatchGetImage",
          "ecr:BatchCheckLayerAvailability",
          "ecr:GetAuthorizationToken"
        ]
        Effect = "Allow"
        Resource = "*"
      }
    ]
  })
}

# CloudWatch Log Group
resource "aws_cloudwatch_log_group" "app_logs" {
  name              = "/aws/ec2/${var.project_name}"
  retention_in_days = 30

  tags = var.common_tags
}

# Route53 DNS Records
resource "aws_route53_record" "app" {
  zone_id = var.route53_zone_id
  name    = var.domain_name
  type    = "A"

  alias {
    name                   = aws_lb.app_lb.dns_name
    zone_id                = aws_lb.app_lb.zone_id
    evaluate_target_health = true
  }
}

# CloudFront Distribution for global content delivery
resource "aws_cloudfront_distribution" "app_distribution" {
  enabled             = true
  is_ipv6_enabled     = true
  comment             = "${var.project_name} distribution"
  default_root_object = "index.html"
  price_class         = "PriceClass_100"  # Use only North America and Europe for cost savings
  
  aliases = [var.domain_name]

  origin {
    domain_name = aws_lb.app_lb.dns_name
    origin_id   = "ALB"

    custom_origin_config {
      http_port              = 80
      https_port             = 443
      origin_protocol_policy = "https-only"
      origin_ssl_protocols   = ["TLSv1.2"]
    }
  }

  default_cache_behavior {
    allowed_methods  = ["DELETE", "GET", "HEAD", "OPTIONS", "PATCH", "POST", "PUT"]
    cached_methods   = ["GET", "HEAD"]
    target_origin_id = "ALB"

    forwarded_values {
      query_string = true
      cookies {
        forward = "all"
      }
      headers = ["Host", "Origin", "Authorization"]
    }

    viewer_protocol_policy = "redirect-to-https"
    min_ttl                = 0
    default_ttl            = 3600
    max_ttl                = 86400
  }

  viewer_certificate {
    acm_certificate_arn      = var.certificate_arn
    ssl_support_method       = "sni-only"
    minimum_protocol_version = "TLSv1.2_2021"
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  tags = var.common_tags
}

# AWS Budget for cost monitoring
resource "aws_budgets_budget" "monthly" {
  name              = "${var.project_name}-monthly-budget"
  budget_type       = "COST"
  limit_amount      = var.monthly_budget_limit
  limit_unit        = "USD"
  time_unit         = "MONTHLY"
  time_period_start = "2023-01-01_00:00"

  notification {
    comparison_operator        = "GREATER_THAN"
    threshold                  = 80
    threshold_type             = "PERCENTAGE"
    notification_type          = "ACTUAL"
    subscriber_email_addresses = var.budget_notification_emails
  }

  notification {
    comparison_operator        = "GREATER_THAN"
    threshold                  = 100
    threshold_type             = "PERCENTAGE"
    notification_type          = "ACTUAL"
    subscriber_email_addresses = var.budget_notification_emails
  }
}

# Data source for current AWS account ID
data "aws_caller_identity" "current" {} 