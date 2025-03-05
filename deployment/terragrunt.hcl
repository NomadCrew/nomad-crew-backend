# Root Terragrunt configuration

# Configure Terragrunt to automatically store tfstate files in an S3 bucket
remote_state {
  backend = "s3"
  config = {
    bucket         = "nomadcrew-terraform-state"
    key            = "${path_relative_to_include()}/terraform.tfstate"
    region         = "us-east-2"
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

# Global variables
inputs = {
  aws_region = "us-east-2"  # Default to US East (N. Virginia) for free tier benefits
  
  # Common tags for all resources
  common_tags = {
    Project     = "NomadCrew"
    ManagedBy   = "Terragrunt"
  }
} 