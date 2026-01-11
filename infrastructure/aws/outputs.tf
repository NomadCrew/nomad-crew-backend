# AWS Infrastructure Outputs

# -----------------------------------------------------------------------------
# Instance Information
# -----------------------------------------------------------------------------

output "instance_id" {
  description = "EC2 instance ID"
  value       = aws_instance.backend.id
}

output "instance_public_ip" {
  description = "Elastic IP address (stable)"
  value       = aws_eip.backend.public_ip
}

output "instance_private_ip" {
  description = "Private IP address within VPC"
  value       = aws_instance.backend.private_ip
}

output "instance_type" {
  description = "Instance type"
  value       = aws_instance.backend.instance_type
}

# -----------------------------------------------------------------------------
# Connection Information
# -----------------------------------------------------------------------------

output "ssh_command" {
  description = "SSH command to connect to the instance"
  value       = "ssh -i ${var.ssh_public_key_path} ubuntu@${aws_eip.backend.public_ip}"
}

output "scp_command" {
  description = "SCP command template for copying files"
  value       = "scp -i ${var.ssh_public_key_path} <local-file> ubuntu@${aws_eip.backend.public_ip}:~/"
}

# -----------------------------------------------------------------------------
# Network Information
# -----------------------------------------------------------------------------

output "vpc_id" {
  description = "VPC ID"
  value       = aws_vpc.main.id
}

output "subnet_id" {
  description = "Public subnet ID"
  value       = aws_subnet.public.id
}

output "security_group_id" {
  description = "Security group ID"
  value       = aws_security_group.backend.id
}

# -----------------------------------------------------------------------------
# AMI Information
# -----------------------------------------------------------------------------

output "ami_id" {
  description = "AMI ID used for the instance"
  value       = data.aws_ami.ubuntu_arm.id
}

output "ami_name" {
  description = "AMI name"
  value       = data.aws_ami.ubuntu_arm.name
}

# -----------------------------------------------------------------------------
# Cost Estimate
# -----------------------------------------------------------------------------

output "estimated_monthly_cost" {
  description = "Estimated monthly cost (approximate)"
  value       = <<-EOT
    Instance (${var.instance_type}):  ~$${var.instance_type == "t4g.small" ? "12" : var.instance_type == "t4g.medium" ? "24" : "?"}/month
    EBS (${var.root_volume_size}GB gp3):       ~$${var.root_volume_size * 0.08}/month
    Elastic IP:              $0 (free while attached)
    Data transfer:           Varies with usage
    ─────────────────────────────────
    Total:                   ~$${var.instance_type == "t4g.small" ? "14" : var.instance_type == "t4g.medium" ? "26" : "?"}/month
  EOT
}

# -----------------------------------------------------------------------------
# Summary
# -----------------------------------------------------------------------------

output "summary" {
  description = "Deployment summary"
  value       = <<-EOT

    ╔══════════════════════════════════════════════════════════════════╗
    ║              NomadCrew AWS Infrastructure                        ║
    ╠══════════════════════════════════════════════════════════════════╣
    ║  Instance IP:    ${aws_eip.backend.public_ip}
    ║  Instance Type:  ${aws_instance.backend.instance_type} (ARM Graviton)
    ║  Region:         ${var.region}
    ║  OS:             Ubuntu 22.04 ARM64
    ╠══════════════════════════════════════════════════════════════════╣
    ║  SSH Access:     ssh ubuntu@${aws_eip.backend.public_ip}
    ║  Open Ports:     22 (SSH), 80 (HTTP), 443 (HTTPS)
    ╠══════════════════════════════════════════════════════════════════╣
    ║  Estimated Cost: ~$14-26/month                                   ║
    ╠══════════════════════════════════════════════════════════════════╣
    ║  Next Steps:                                                     ║
    ║  1. SSH to instance                                              ║
    ║  2. Proceed to Phase 14: Coolify Installation                    ║
    ╚══════════════════════════════════════════════════════════════════╝

  EOT
}
