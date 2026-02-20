# Oracle Cloud Infrastructure - NomadCrew Backend
# Always Free ARM Ampere A1 Flex Instance
#
# Resources created:
# - Compartment for NomadCrew resources
# - VCN with public subnet
# - Internet Gateway and Route Table
# - Security List (ports 22, 80, 443)
# - ARM compute instance (4 OCPU, 24 GB RAM)

terraform {
  required_version = ">= 1.5.0"
  required_providers {
    oci = {
      source  = "oracle/oci"
      version = ">= 6.0.0"
    }
  }
}

provider "oci" {
  tenancy_ocid     = var.tenancy_ocid
  user_ocid        = var.user_ocid
  fingerprint      = var.fingerprint
  private_key_path = var.private_key_path
  region           = var.region
}

# -----------------------------------------------------------------------------
# Data Sources
# -----------------------------------------------------------------------------

# Get the availability domain in the region
data "oci_identity_availability_domain" "ad" {
  compartment_id = var.tenancy_ocid
  ad_number      = var.availability_domain_number
}

# Get the latest Ubuntu 22.04 ARM image
data "oci_core_images" "ubuntu_arm" {
  compartment_id           = var.tenancy_ocid
  operating_system         = "Canonical Ubuntu"
  operating_system_version = "22.04"
  shape                    = "VM.Standard.A1.Flex"
  sort_by                  = "TIMECREATED"
  sort_order               = "DESC"
}

# -----------------------------------------------------------------------------
# Compartment
# -----------------------------------------------------------------------------

resource "oci_identity_compartment" "nomadcrew" {
  compartment_id = var.tenancy_ocid
  name           = "nomadcrew"
  description    = "NomadCrew production resources"
}

# -----------------------------------------------------------------------------
# Networking - VCN
# -----------------------------------------------------------------------------

resource "oci_core_vcn" "main" {
  compartment_id = oci_identity_compartment.nomadcrew.id
  cidr_blocks    = ["10.0.0.0/16"]
  display_name   = "nomadcrew-vcn"
  dns_label      = "nomadcrew"
}

# -----------------------------------------------------------------------------
# Networking - Internet Gateway
# -----------------------------------------------------------------------------

resource "oci_core_internet_gateway" "main" {
  compartment_id = oci_identity_compartment.nomadcrew.id
  vcn_id         = oci_core_vcn.main.id
  display_name   = "nomadcrew-igw"
  enabled        = true
}

# -----------------------------------------------------------------------------
# Networking - Route Table
# -----------------------------------------------------------------------------

resource "oci_core_route_table" "public" {
  compartment_id = oci_identity_compartment.nomadcrew.id
  vcn_id         = oci_core_vcn.main.id
  display_name   = "public-route-table"

  route_rules {
    destination       = "0.0.0.0/0"
    destination_type  = "CIDR_BLOCK"
    network_entity_id = oci_core_internet_gateway.main.id
  }
}

# -----------------------------------------------------------------------------
# Networking - Security List
# -----------------------------------------------------------------------------

resource "oci_core_security_list" "main" {
  compartment_id = oci_identity_compartment.nomadcrew.id
  vcn_id         = oci_core_vcn.main.id
  display_name   = "nomadcrew-security-list"

  # Allow all outbound traffic
  egress_security_rules {
    destination = "0.0.0.0/0"
    protocol    = "all"
    stateless   = false
  }

  # SSH (port 22)
  ingress_security_rules {
    protocol  = "6" # TCP
    source    = "0.0.0.0/0"
    stateless = false

    tcp_options {
      min = 22
      max = 22
    }
  }

  # HTTP (port 80)
  ingress_security_rules {
    protocol  = "6" # TCP
    source    = "0.0.0.0/0"
    stateless = false

    tcp_options {
      min = 80
      max = 80
    }
  }

  # HTTPS (port 443)
  ingress_security_rules {
    protocol  = "6" # TCP
    source    = "0.0.0.0/0"
    stateless = false

    tcp_options {
      min = 443
      max = 443
    }
  }

  # Coolify Dashboard (port 8000) — NOT exposed publicly.
  # Access via SSH tunnel: ssh -L 8000:localhost:8000 ubuntu@84.235.242.52
  # Then open http://localhost:8000 in your browser.

  # ICMP Type 3 Code 4 (Path MTU Discovery)
  ingress_security_rules {
    protocol  = "1" # ICMP
    source    = "0.0.0.0/0"
    stateless = false

    icmp_options {
      type = 3
      code = 4
    }
  }

  # ICMP Type 3 (Destination Unreachable - for debugging)
  ingress_security_rules {
    protocol  = "1" # ICMP
    source    = "10.0.0.0/16"
    stateless = false

    icmp_options {
      type = 3
    }
  }
}

# -----------------------------------------------------------------------------
# Networking - Public Subnet
# -----------------------------------------------------------------------------

resource "oci_core_subnet" "public" {
  compartment_id             = oci_identity_compartment.nomadcrew.id
  vcn_id                     = oci_core_vcn.main.id
  cidr_block                 = "10.0.0.0/24"
  display_name               = "public-subnet"
  dns_label                  = "public"
  route_table_id             = oci_core_route_table.public.id
  security_list_ids          = [oci_core_security_list.main.id]
  prohibit_public_ip_on_vnic = false
}

# -----------------------------------------------------------------------------
# Compute - ARM Instance (Always Free)
# -----------------------------------------------------------------------------

resource "oci_core_instance" "backend" {
  availability_domain = data.oci_identity_availability_domain.ad.name
  compartment_id      = oci_identity_compartment.nomadcrew.id
  display_name        = "nomadcrew-backend"
  shape               = "VM.Standard.A1.Flex"

  # Always Free tier: 4 OCPU, 24 GB RAM total
  shape_config {
    ocpus         = 4
    memory_in_gbs = 24
  }

  source_details {
    source_type             = "image"
    source_id               = data.oci_core_images.ubuntu_arm.images[0].id
    boot_volume_size_in_gbs = 50 # Default boot volume size
  }

  create_vnic_details {
    subnet_id                 = oci_core_subnet.public.id
    assign_public_ip          = true
    display_name              = "nomadcrew-backend-vnic"
    hostname_label            = "backend"
    skip_source_dest_check    = false
    assign_private_dns_record = true
  }

  metadata = {
    ssh_authorized_keys = file(var.ssh_public_key_path)
  }

  # Prevent accidental deletion
  preserve_boot_volume = true

  # Use standard launch options for ARM
  launch_options {
    boot_volume_type                    = "PARAVIRTUALIZED"
    firmware                            = "UEFI_64"
    network_type                        = "PARAVIRTUALIZED"
    remote_data_volume_type             = "PARAVIRTUALIZED"
    is_pv_encryption_in_transit_enabled = true
  }
}
