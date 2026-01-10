# Oracle Cloud Infrastructure Outputs
# Values available after terraform apply

# -----------------------------------------------------------------------------
# Instance Information
# -----------------------------------------------------------------------------

output "instance_public_ip" {
  description = "Public IP address of the NomadCrew backend instance"
  value       = oci_core_instance.backend.public_ip
}

output "instance_id" {
  description = "OCID of the compute instance"
  value       = oci_core_instance.backend.id
}

output "instance_private_ip" {
  description = "Private IP address of the instance within the VCN"
  value       = oci_core_instance.backend.private_ip
}

output "instance_state" {
  description = "Current state of the instance (RUNNING, STOPPED, etc.)"
  value       = oci_core_instance.backend.state
}

# -----------------------------------------------------------------------------
# Connection Information
# -----------------------------------------------------------------------------

output "ssh_command" {
  description = "SSH command to connect to the instance"
  value       = "ssh -i ${var.ssh_public_key_path} ubuntu@${oci_core_instance.backend.public_ip}"
}

output "scp_command" {
  description = "SCP command template for copying files to the instance"
  value       = "scp -i ${var.ssh_public_key_path} <local-file> ubuntu@${oci_core_instance.backend.public_ip}:~/"
}

# -----------------------------------------------------------------------------
# Network Information
# -----------------------------------------------------------------------------

output "vcn_id" {
  description = "OCID of the Virtual Cloud Network"
  value       = oci_core_vcn.main.id
}

output "subnet_id" {
  description = "OCID of the public subnet"
  value       = oci_core_subnet.public.id
}

output "compartment_id" {
  description = "OCID of the NomadCrew compartment"
  value       = oci_identity_compartment.nomadcrew.id
}

# -----------------------------------------------------------------------------
# Image Information
# -----------------------------------------------------------------------------

output "instance_image" {
  description = "Name of the OS image used for the instance"
  value       = data.oci_core_images.ubuntu_arm.images[0].display_name
}

# -----------------------------------------------------------------------------
# Summary
# -----------------------------------------------------------------------------

output "summary" {
  description = "Quick summary of deployed resources"
  value = <<-EOT

    ╔══════════════════════════════════════════════════════════════════╗
    ║              NomadCrew Oracle Cloud Infrastructure               ║
    ╠══════════════════════════════════════════════════════════════════╣
    ║  Instance IP:    ${oci_core_instance.backend.public_ip}
    ║  Instance Shape: VM.Standard.A1.Flex (4 OCPU, 24 GB RAM)
    ║  Region:         ${var.region}
    ║  OS:             Ubuntu 22.04 ARM64
    ╠══════════════════════════════════════════════════════════════════╣
    ║  SSH Access:     ssh ubuntu@${oci_core_instance.backend.public_ip}
    ║  Open Ports:     22 (SSH), 80 (HTTP), 443 (HTTPS)
    ╠══════════════════════════════════════════════════════════════════╣
    ║  Next Steps:                                                     ║
    ║  1. SSH to instance and run setup-firewall.sh                    ║
    ║  2. Proceed to Phase 14: Coolify Installation                    ║
    ╚══════════════════════════════════════════════════════════════════╝

  EOT
}
