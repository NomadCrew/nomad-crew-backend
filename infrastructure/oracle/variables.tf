# Oracle Cloud Infrastructure Variables
# Required for OCI Provider authentication and resource configuration

# -----------------------------------------------------------------------------
# Authentication Variables (Required)
# -----------------------------------------------------------------------------

variable "tenancy_ocid" {
  description = "OCID of your OCI tenancy. Find at: OCI Console > Profile > Tenancy"
  type        = string
}

variable "user_ocid" {
  description = "OCID of the user calling the API. Find at: OCI Console > Profile > My Profile"
  type        = string
}

variable "fingerprint" {
  description = "Fingerprint of the API signing key. Shown when you add an API key to your user profile"
  type        = string
}

variable "private_key_path" {
  description = "Path to the private key file for API authentication"
  type        = string
  default     = "~/.oci/oci_api_key.pem"
}

# -----------------------------------------------------------------------------
# Region Configuration
# -----------------------------------------------------------------------------

variable "region" {
  description = "OCI region identifier. Must match your home region. Common values: us-ashburn-1, uk-london-1, me-dubai-1"
  type        = string
  default     = "me-dubai-1"
}

# -----------------------------------------------------------------------------
# SSH Configuration
# -----------------------------------------------------------------------------

variable "ssh_public_key_path" {
  description = "Path to SSH public key for instance access. Will be added to ubuntu user's authorized_keys"
  type        = string
  default     = "~/.ssh/id_rsa.pub"
}

# -----------------------------------------------------------------------------
# Instance Configuration (Optional - defaults to Always Free tier)
# -----------------------------------------------------------------------------

variable "instance_ocpus" {
  description = "Number of OCPUs for the instance. Always Free tier allows up to 4 total"
  type        = number
  default     = 4
}

variable "instance_memory_gb" {
  description = "Memory in GB for the instance. Always Free tier allows up to 24 GB total"
  type        = number
  default     = 24
}

variable "boot_volume_size_gb" {
  description = "Boot volume size in GB. Always Free includes up to 200 GB total"
  type        = number
  default     = 50
}

variable "availability_domain_number" {
  description = "Availability Domain number (1, 2, or 3). Try different ADs if 'Out of capacity' error occurs"
  type        = number
  default     = 1
}
