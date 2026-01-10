# Oracle Cloud Infrastructure - NomadCrew Backend

This directory contains Terraform configuration for deploying NomadCrew backend infrastructure on Oracle Cloud's Always Free tier.

## Resources Created

- **Compartment**: `nomadcrew` - isolated container for all resources
- **VCN**: `nomadcrew-vcn` (10.0.0.0/16) - Virtual Cloud Network
- **Subnet**: `public-subnet` (10.0.0.0/24) - Public subnet with internet access
- **Internet Gateway**: `nomadcrew-igw` - Enables outbound internet connectivity
- **Security List**: Allows inbound traffic on ports 22, 80, 443
- **Compute Instance**: ARM Ampere A1 Flex (4 OCPU, 24 GB RAM) - Ubuntu 22.04

## Prerequisites

1. **Oracle Cloud Account** with Pay-As-You-Go billing (free upgrade, prevents capacity issues)
2. **Terraform** >= 1.5.0
3. **OCI API Key** configured in `~/.oci/`
4. **SSH Key** for instance access

## Setup Instructions

### 1. Configure OCI API Credentials

```bash
# Generate API key pair
mkdir -p ~/.oci
openssl genrsa -out ~/.oci/oci_api_key.pem 2048
chmod 600 ~/.oci/oci_api_key.pem
openssl rsa -pubout -in ~/.oci/oci_api_key.pem -out ~/.oci/oci_api_key_public.pem

# Add public key to OCI Console:
# Profile > My Profile > API Keys > Add API Key
```

### 2. Create terraform.tfvars

```bash
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars with your OCI credentials
```

Required values (from OCI Console):
- `tenancy_ocid` - Profile > Tenancy > OCID
- `user_ocid` - Profile > My Profile > OCID
- `fingerprint` - Shown when adding API key
- `private_key_path` - Path to `oci_api_key.pem`
- `region` - Your home region (e.g., `me-dubai-1`)
- `ssh_public_key_path` - Path to SSH public key

### 3. Deploy Infrastructure

```bash
terraform init
terraform plan
terraform apply
```

### 4. Handle "Out of Capacity" Error

ARM instances are popular. If you get capacity errors:

```bash
# Option 1: Retry manually
terraform apply

# Option 2: Use retry script
./retry-instance.sh

# Option 3: Use automated tools
# https://github.com/hitrov/oci-arm-host-capacity
```

### 5. Configure OS Firewall

After instance is running, configure iptables:

```bash
# Copy and run firewall script
scp scripts/setup-firewall.sh ubuntu@<INSTANCE_IP>:~
ssh ubuntu@<INSTANCE_IP> "chmod +x setup-firewall.sh && sudo ./setup-firewall.sh"
```

## SSH Access

```bash
# Get SSH command from Terraform output
terraform output ssh_command

# Or manually
ssh -i ~/.ssh/id_rsa ubuntu@<INSTANCE_IP>
```

## Ports Open

| Port | Protocol | Purpose |
|------|----------|---------|
| 22   | TCP      | SSH access |
| 80   | TCP      | HTTP (Coolify, redirects to HTTPS) |
| 443  | TCP      | HTTPS (Application traffic) |

**Note**: Both OCI Security List AND OS iptables must allow traffic. The Security List is configured by Terraform; iptables is configured by `setup-firewall.sh`.

## File Structure

```
infrastructure/oracle/
├── main.tf                  # Main Terraform resources
├── variables.tf             # Input variables
├── outputs.tf               # Output values
├── terraform.tfvars.example # Example configuration
├── terraform.tfvars         # Your configuration (git-ignored)
├── retry-instance.sh        # Capacity retry script
├── scripts/
│   └── setup-firewall.sh    # OS firewall configuration
└── README.md                # This file
```

## Costs

All resources are within Oracle Cloud's **Always Free** tier:
- 4 OCPU ARM Ampere A1
- 24 GB RAM
- 200 GB block storage
- 10 TB/month outbound data

**$0/month** if staying within limits.

## Next Steps

After infrastructure is deployed:
1. **Phase 14**: Install Coolify on the instance
2. **Phase 15**: Configure CI/CD pipeline
3. **Phase 16**: Deploy NomadCrew application

## Troubleshooting

### "Out of host capacity"
ARM instances are in high demand. Solutions:
1. Keep retrying (capacity appears randomly)
2. Use `retry-instance.sh` script
3. Upgrade to Pay-As-You-Go for priority (free, no charges within limits)

### Cannot SSH to instance
1. Check Security List allows port 22
2. Verify SSH key matches what was configured
3. Check instance is in RUNNING state: `terraform output instance_state`

### Ports 80/443 not reachable
1. Check Security List (Terraform handles this)
2. Check OS firewall: `sudo iptables -L INPUT -n`
3. Run `setup-firewall.sh` if iptables not configured
