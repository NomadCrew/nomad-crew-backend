# AWS Infrastructure Variables

# -----------------------------------------------------------------------------
# Region Configuration
# -----------------------------------------------------------------------------

variable "region" {
  description = "AWS region to deploy resources"
  type        = string
  default     = "us-east-2"
}

# -----------------------------------------------------------------------------
# Instance Configuration
# -----------------------------------------------------------------------------

variable "instance_type" {
  description = "EC2 instance type. ARM Graviton instances (t4g.*) are most cost-effective"
  type        = string
  default     = "t4g.small" # 2 vCPU, 2 GB RAM - ~$12/month

  # Cost comparison (us-east-2, on-demand):
  # t4g.micro:  1 vCPU, 1 GB  - $6/month   (too small for Coolify)
  # t4g.small:  2 vCPU, 2 GB  - $12/month  (minimum recommended)
  # t4g.medium: 2 vCPU, 4 GB  - $24/month  (comfortable for Coolify + app)
  # t4g.large:  2 vCPU, 8 GB  - $48/month  (overkill)
}

variable "root_volume_size" {
  description = "Root EBS volume size in GB"
  type        = number
  default     = 30 # ~$2.40/month for gp3
}

# -----------------------------------------------------------------------------
# SSH Configuration
# -----------------------------------------------------------------------------

variable "ssh_public_key_path" {
  description = "Path to SSH public key for EC2 access"
  type        = string
  default     = "~/.ssh/id_rsa.pub"
}

variable "ssh_allowed_cidrs" {
  description = "CIDR blocks allowed for SSH access. Use your IP for security, or 0.0.0.0/0 for open access"
  type        = list(string)
  default     = ["0.0.0.0/0"] # Consider restricting to your IP
}

# -----------------------------------------------------------------------------
# Tags
# -----------------------------------------------------------------------------

variable "environment" {
  description = "Environment name for tagging"
  type        = string
  default     = "production"
}
