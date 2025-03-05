# Dev environment configuration

# Include all settings from the root terragrunt.hcl file
include {
  path = find_in_parent_folders()
}

# Terraform source
terraform {
  source = "../../terraform"
}

# Dev-specific inputs
inputs = {
  environment = "dev"
  
  # VPC Configuration
  vpc_cidr             = "10.0.0.0/16"
  availability_zones   = ["us-east-2a", "us-east-2b"]
  private_subnet_cidrs = ["10.0.1.0/24", "10.0.2.0/24"]
  public_subnet_cidrs  = ["10.0.101.0/24", "10.0.102.0/24"]
  
  # Application Configuration
  app_port = 8080
  
  # Database Configuration
  db_name     = "nomadcrew"
  db_username = "postgres"
  # db_password is set via environment variable TF_VAR_db_password
  
  # Auto Scaling Configuration
  min_size         = 1
  max_size         = 2
  desired_capacity = 1
  
  # Domain Configuration
  domain_name = "dev-api.nomadcrew.uk"
  # route53_zone_id and certificate_arn are set via environment variables
  
  # Budget Configuration
  monthly_budget_limit = 50
  budget_notification_emails = ["admin@nomadcrew.uk"]
  
  # Common Tags
  common_tags = {
    Project     = "NomadCrew"
    Environment = "dev"
    ManagedBy   = "Terragrunt"
  }
} 