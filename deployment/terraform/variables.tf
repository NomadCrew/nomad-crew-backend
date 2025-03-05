variable "aws_region" {
  description = "The AWS region to deploy resources"
  type        = string
  default     = "us-east-1"  # Default to US East (N. Virginia) for free tier benefits
}

variable "project_name" {
  description = "The name of the project"
  type        = string
  default     = "nomadcrew"
}

variable "environment" {
  description = "The deployment environment (e.g., dev, staging, prod)"
  type        = string
  default     = "dev"
}

variable "vpc_cidr" {
  description = "The CIDR block for the VPC"
  type        = string
  default     = "10.0.0.0/16"
}

variable "availability_zones" {
  description = "The availability zones to deploy resources"
  type        = list(string)
  default     = ["us-east-1a", "us-east-1b"]
}

variable "private_subnet_cidrs" {
  description = "The CIDR blocks for the private subnets"
  type        = list(string)
  default     = ["10.0.3.0/24", "10.0.4.0/24"]
}

variable "public_subnet_cidrs" {
  description = "The CIDR blocks for the public subnets"
  type        = list(string)
  default     = ["10.0.1.0/24", "10.0.2.0/24"]
}

variable "app_port" {
  description = "The port the application listens on"
  type        = number
  default     = 8080
}

variable "ssh_allowed_cidrs" {
  description = "The CIDR blocks allowed to SSH to the instances"
  type        = list(string)
  default     = ["0.0.0.0/0"]  # Consider restricting this in production
}

variable "db_username" {
  description = "The username for the database"
  type        = string
  default     = "postgres"
  sensitive   = true
}

variable "db_password" {
  description = "The password for the database"
  type        = string
  sensitive   = true
}

variable "db_name" {
  description = "The name of the database"
  type        = string
  default     = "nomadcrew"
}

variable "ami_id" {
  description = "The AMI ID to use for the EC2 instances"
  type        = string
  # Default to Amazon Linux 2023 in us-east-1
  default     = ""
}

variable "nat_instance_ami" {
  description = "The AMI ID to use for the NAT instance"
  type        = string
  default     = ""  # Will use the latest Amazon Linux 2 AMI if not specified
}

variable "redis_password" {
  description = "The password for Redis"
  type        = string
  default     = "redispass"  # Consider using a more secure password in production
  sensitive   = true
}

variable "key_name" {
  description = "The name of the key pair to use for SSH access"
  type        = string
}

variable "domain_name" {
  description = "The domain name for the application"
  type        = string
  default     = "api.nomadcrew.uk"
}

variable "route53_zone_id" {
  description = "The Route 53 hosted zone ID for the domain"
  type        = string
}

variable "certificate_arn" {
  description = "The ARN of the ACM certificate for the domain"
  type        = string
}

variable "aws_secrets_path" {
  description = "The path to the AWS Secrets Manager secrets"
  type        = string
  default     = "nomadcrew/dev/secrets"
}

variable "monthly_budget_limit" {
  description = "The monthly budget limit in USD"
  type        = number
  default     = 50  # Set a reasonable budget for a small application
}

variable "budget_notification_emails" {
  description = "The email addresses to notify for budget alerts"
  type        = list(string)
  default     = []
}

variable "common_tags" {
  description = "Common tags to apply to all resources"
  type        = map(string)
  default = {
    Project     = "NomadCrew"
    Environment = "dev"
    ManagedBy   = "Terraform"
  }
}

variable "ecr_repository_uri" {
  description = "The URI of the ECR repository"
  type        = string
  default     = null
}

variable "image_tag" {
  description = "The tag of the Docker image to deploy"
  type        = string
  default     = "latest"
} 