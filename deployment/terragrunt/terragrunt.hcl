# Terragrunt configuration
remote_state {
  backend = "s3"
  config = {
    bucket         = "nomadcrew-terraform-state"
    key            = "${path_relative_to_include()}/terraform.tfstate"
    region         = "us-east-1"
    encrypt        = true
    dynamodb_table = "nomadcrew-terraform-locks"
  }
  generate = {
    path      = "backend.tf"
    if_exists = "overwrite_terragrunt"
  }
}

# Generate provider configuration
generate "provider" {
  path      = "provider.tf"
  if_exists = "overwrite_terragrunt"
  contents  = <<EOF
provider "aws" {
  region = var.aws_region
}
EOF
}

# Include all settings from the root terragrunt.hcl file
include {
  path = find_in_parent_folders()
}

# Inputs for the Terraform module
inputs = {
  project_name = "nomadcrew"
  environment  = "dev"
  
  # VPC Configuration
  vpc_cidr             = "10.0.0.0/16"
  availability_zones   = ["us-east-1a", "us-east-1b"]
  private_subnet_cidrs = ["10.0.1.0/24", "10.0.2.0/24"]
  public_subnet_cidrs  = ["10.0.101.0/24", "10.0.102.0/24"]
  
  # Application Configuration
  app_port = 8080
  
  # Database Configuration
  db_name     = "nomadcrew"
  db_username = "postgres"
  # db_password is set via environment variable TF_VAR_db_password
  
  # Domain Configuration
  domain_name = "api.nomadcrew.uk"
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