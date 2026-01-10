# Phase 13: Oracle Cloud Setup - Research

**Researched:** 2026-01-10
**Domain:** Oracle Cloud Infrastructure (OCI) Always Free tier setup
**Confidence:** HIGH

<research_summary>
## Summary

Researched the Oracle Cloud Infrastructure ecosystem for setting up Always Free ARM compute instances. The standard approach involves creating an OCI account, provisioning an ARM Ampere A1 Flex instance (4 OCPU, 24 GB RAM), and configuring VCN networking with proper security lists.

Key findings:
1. **"Out of Capacity" is the primary challenge** - ARM instances are in high demand. Solution: Upgrade to Pay-As-You-Go (free, but gets priority) or use automated retry scripts.
2. **Two-layer firewall configuration required** - Both OCI Security Lists AND OS-level iptables must be configured for ports to work.
3. **Idle instance reclamation policy** - Instances with <20% CPU/Network/Memory usage over 7 days may be reclaimed. Keep instances active.
4. **Region selection matters** - Ashburn, Chicago, London, Sydney have better ARM availability than San Jose, Tokyo, Singapore, Amsterdam.

**Primary recommendation:** Sign up with a region known for ARM availability (Ashburn or London), immediately upgrade to Pay-As-You-Go (no charges if staying within free limits), and use Terraform for reproducible infrastructure.
</research_summary>

<standard_stack>
## Standard Stack

### Core
| Tool | Version | Purpose | Why Standard |
|------|---------|---------|--------------|
| OCI Console | N/A | Web-based management interface | Primary way to create account and initial setup |
| OCI CLI | Latest | Command-line infrastructure management | Automation, scripting, "out of capacity" workarounds |
| Terraform | >= 1.5 | Infrastructure as Code | Reproducible deployments, documented configuration |
| OCI Terraform Provider | >= 6.0 | OCI-specific Terraform resources | Official Oracle provider |

### Supporting
| Tool | Version | Purpose | When to Use |
|------|---------|---------|-------------|
| Cloud Shell | N/A | Browser-based CLI with pre-auth | Quick tasks without local setup |
| ssh-keygen | N/A | SSH key generation | Instance access authentication |
| iptables | N/A | OS-level firewall | Required alongside Security Lists |
| iptables-persistent | N/A | Persist iptables rules | Keep firewall rules across reboots |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Terraform | OCI Console (manual) | Console is faster for one-off setup but not reproducible |
| Terraform | OCI CLI scripts | CLI works but Terraform has better state management |
| iptables | ufw | ufw is simpler but iptables is pre-installed on Oracle Linux |

**Installation:**
```bash
# Terraform
brew install terraform  # macOS
choco install terraform # Windows

# OCI CLI
bash -c "$(curl -L https://raw.githubusercontent.com/oracle/oci-cli/master/scripts/install/install.sh)"
```
</standard_stack>

<architecture_patterns>
## Architecture Patterns

### Recommended OCI Resource Structure
```
Tenancy (root)
├── Compartment: nomadcrew-prod
│   ├── VCN: nomadcrew-vcn (10.0.0.0/16)
│   │   ├── Subnet: public-subnet (10.0.0.0/24)
│   │   │   └── Compute: nomadcrew-backend
│   │   ├── Internet Gateway: nomadcrew-igw
│   │   ├── Route Table: public-route-table
│   │   └── Security List: nomadcrew-security-list
│   └── Block Volume: (included in boot volume)
└── Compartment: sandbox (for testing)
```

### Pattern 1: Single ARM Instance for Small Backend
**What:** One VM.Standard.A1.Flex with all 4 OCPUs and 24 GB RAM
**When to use:** Single application deployment (like NomadCrew backend)
**Example:**
```hcl
# Source: OCI Terraform Provider docs
resource "oci_core_instance" "backend" {
  availability_domain = data.oci_identity_availability_domain.ad.name
  compartment_id      = var.compartment_id
  shape               = "VM.Standard.A1.Flex"

  shape_config {
    ocpus         = 4      # All 4 free OCPUs
    memory_in_gbs = 24     # All 24 GB free memory
  }

  source_details {
    source_type = "image"
    source_id   = data.oci_core_images.ubuntu.images[0].id
  }

  create_vnic_details {
    subnet_id        = oci_core_subnet.public.id
    assign_public_ip = true
  }

  metadata = {
    ssh_authorized_keys = file("~/.ssh/id_rsa.pub")
  }
}
```

### Pattern 2: VCN with Public Subnet
**What:** Standard networking setup for internet-facing application
**When to use:** Any web application needing public access
**Example:**
```hcl
# Source: OCI Terraform docs
resource "oci_core_vcn" "main" {
  compartment_id = var.compartment_id
  cidr_block     = "10.0.0.0/16"
  display_name   = "nomadcrew-vcn"
  dns_label      = "nomadcrew"
}

resource "oci_core_internet_gateway" "igw" {
  compartment_id = var.compartment_id
  vcn_id         = oci_core_vcn.main.id
  display_name   = "nomadcrew-igw"
}

resource "oci_core_route_table" "public" {
  compartment_id = var.compartment_id
  vcn_id         = oci_core_vcn.main.id

  route_rules {
    destination       = "0.0.0.0/0"
    network_entity_id = oci_core_internet_gateway.igw.id
  }
}

resource "oci_core_subnet" "public" {
  compartment_id      = var.compartment_id
  vcn_id              = oci_core_vcn.main.id
  cidr_block          = "10.0.0.0/24"
  display_name        = "public-subnet"
  dns_label           = "public"
  route_table_id      = oci_core_route_table.public.id
  security_list_ids   = [oci_core_security_list.main.id]
}
```

### Pattern 3: Security List for Web Application
**What:** Ingress rules for SSH, HTTP, HTTPS
**When to use:** Standard web application deployment
**Example:**
```hcl
# Source: OCI Security List docs
resource "oci_core_security_list" "main" {
  compartment_id = var.compartment_id
  vcn_id         = oci_core_vcn.main.id
  display_name   = "nomadcrew-security-list"

  # Allow all outbound traffic
  egress_security_rules {
    destination = "0.0.0.0/0"
    protocol    = "all"
  }

  # SSH access
  ingress_security_rules {
    protocol = "6"  # TCP
    source   = "0.0.0.0/0"  # Consider restricting to your IP
    tcp_options {
      min = 22
      max = 22
    }
  }

  # HTTP
  ingress_security_rules {
    protocol = "6"
    source   = "0.0.0.0/0"
    tcp_options {
      min = 80
      max = 80
    }
  }

  # HTTPS
  ingress_security_rules {
    protocol = "6"
    source   = "0.0.0.0/0"
    tcp_options {
      min = 443
      max = 443
    }
  }

  # ICMP (ping) - recommended for debugging
  ingress_security_rules {
    protocol = "1"  # ICMP
    source   = "0.0.0.0/0"
    icmp_options {
      type = 3
      code = 4
    }
  }
}
```

### Anti-Patterns to Avoid
- **Only configuring Security Lists:** OS-level firewall (iptables) ALSO blocks traffic by default
- **Opening SSH to 0.0.0.0/0 long-term:** Restrict to your IP range after initial setup
- **Using root compartment:** Always create a dedicated compartment for production resources
- **Ignoring idle policy:** Instance may be reclaimed if underutilized for 7 days
</architecture_patterns>

<dont_hand_roll>
## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| "Out of capacity" retry | Manual console clicking | [oci-arm-host-capacity](https://github.com/hitrov/oci-arm-host-capacity) or Terraform retry script | Capacity appears randomly; automation catches it |
| Infrastructure provisioning | Console-only manual setup | Terraform with OCI provider | Reproducible, documented, version-controlled |
| SSH key management | Shared keys, manual copy | Generate per-instance, store in secure location | Security best practice |
| Security rules | Manual iptables commands | iptables-persistent + Security Lists | Rules survive reboots |
| Instance monitoring | Manual SSH checks | OCI Monitoring + Grafana | Automated alerts, dashboards |

**Key insight:** OCI has mature tooling - the OCI CLI, Terraform provider, and official modules handle the complexity of provisioning. Don't waste time with manual Console workflows for anything you'll need to repeat or reproduce.
</dont_hand_roll>

<common_pitfalls>
## Common Pitfalls

### Pitfall 1: "Out of Host Capacity" Error
**What goes wrong:** Cannot create ARM instance - OCI reports no capacity available
**Why it happens:** ARM free tier is extremely popular; capacity is limited per region
**How to avoid:**
1. **Upgrade to Pay-As-You-Go** (recommended - no charge, better priority)
2. Use automated retry scripts that poll for capacity
3. Try different Availability Domains within your region
4. Choose a less popular home region (Ashburn, London, Sydney better than San Jose, Tokyo)
**Warning signs:** Immediate error on instance creation attempt

### Pitfall 2: Ports Appear Open But Traffic Blocked
**What goes wrong:** Security List shows ports 80/443 open, but can't connect
**Why it happens:** OCI has TWO firewalls - Security Lists AND OS-level iptables
**How to avoid:**
```bash
# After Security List configuration, also run on instance:
sudo iptables -I INPUT -p tcp --dport 80 -j ACCEPT
sudo iptables -I INPUT -p tcp --dport 443 -j ACCEPT
sudo apt-get install iptables-persistent -y
sudo netfilter-persistent save
```
**Warning signs:** `curl localhost:80` works but external access fails

### Pitfall 3: Instance Reclaimed as "Idle"
**What goes wrong:** Instance stopped or deleted by Oracle after 7 days
**Why it happens:** Free tier policy reclaims instances with <20% CPU, Network, Memory utilization
**How to avoid:**
1. **Upgrade to Pay-As-You-Go** (prevents reclamation entirely)
2. Run actual workloads (Coolify + app should keep it busy)
3. Install keepalive/activity scripts if truly idle
**Warning signs:** Email from Oracle about idle instance, instance suddenly stopped

### Pitfall 4: Wrong Home Region Selection
**What goes wrong:** Stuck in a region with perpetual "out of capacity" for ARM
**Why it happens:** Home region cannot be changed after account creation
**How to avoid:**
- Research ARM availability before signup
- Recommended: **US-ASHBURN-1**, **UK-LONDON-1**, **AP-SYDNEY-1**
- Avoid: San Jose, Tokyo, Singapore, Amsterdam (high demand)
**Warning signs:** Cannot create ARM instance after weeks of trying

### Pitfall 5: SSH Keys Lost or Misconfigured
**What goes wrong:** Cannot access instance, locked out permanently
**Why it happens:** SSH key specified at creation is the ONLY way to access
**How to avoid:**
- Store SSH keys securely before creating instance
- Use a descriptive key name you'll remember
- Test SSH access immediately after instance creation
- Document the key location in your infrastructure repo
**Warning signs:** "Permission denied (publickey)" on SSH attempt
</common_pitfalls>

<code_examples>
## Code Examples

### Complete Terraform Configuration for Always Free Instance
```hcl
# Source: Compiled from OCI Terraform Provider docs and terraform-oci-free module

terraform {
  required_version = ">= 1.5.0"
  required_providers {
    oci = {
      source  = "oracle/oci"
      version = ">= 6.0.0"
    }
  }
}

variable "tenancy_ocid" {}
variable "user_ocid" {}
variable "fingerprint" {}
variable "private_key_path" {}
variable "region" { default = "us-ashburn-1" }
variable "ssh_public_key_path" { default = "~/.ssh/id_rsa.pub" }

provider "oci" {
  tenancy_ocid     = var.tenancy_ocid
  user_ocid        = var.user_ocid
  fingerprint      = var.fingerprint
  private_key_path = var.private_key_path
  region           = var.region
}

# Get availability domain
data "oci_identity_availability_domain" "ad" {
  compartment_id = var.tenancy_ocid
  ad_number      = 1
}

# Get Ubuntu ARM image
data "oci_core_images" "ubuntu_arm" {
  compartment_id           = var.tenancy_ocid
  operating_system         = "Canonical Ubuntu"
  operating_system_version = "22.04"
  shape                    = "VM.Standard.A1.Flex"
  sort_by                  = "TIMECREATED"
  sort_order               = "DESC"
}

# Compartment (use root for simplicity, or create dedicated)
resource "oci_identity_compartment" "main" {
  compartment_id = var.tenancy_ocid
  name           = "nomadcrew"
  description    = "NomadCrew production resources"
}

# VCN
resource "oci_core_vcn" "main" {
  compartment_id = oci_identity_compartment.main.id
  cidr_blocks    = ["10.0.0.0/16"]
  display_name   = "nomadcrew-vcn"
  dns_label      = "nomadcrew"
}

# Internet Gateway
resource "oci_core_internet_gateway" "main" {
  compartment_id = oci_identity_compartment.main.id
  vcn_id         = oci_core_vcn.main.id
  display_name   = "nomadcrew-igw"
}

# Route Table
resource "oci_core_route_table" "public" {
  compartment_id = oci_identity_compartment.main.id
  vcn_id         = oci_core_vcn.main.id
  display_name   = "public-route-table"

  route_rules {
    destination       = "0.0.0.0/0"
    network_entity_id = oci_core_internet_gateway.main.id
  }
}

# Security List
resource "oci_core_security_list" "main" {
  compartment_id = oci_identity_compartment.main.id
  vcn_id         = oci_core_vcn.main.id
  display_name   = "nomadcrew-security-list"

  egress_security_rules {
    destination = "0.0.0.0/0"
    protocol    = "all"
  }

  ingress_security_rules {
    protocol = "6"
    source   = "0.0.0.0/0"
    tcp_options { min = 22; max = 22 }
  }

  ingress_security_rules {
    protocol = "6"
    source   = "0.0.0.0/0"
    tcp_options { min = 80; max = 80 }
  }

  ingress_security_rules {
    protocol = "6"
    source   = "0.0.0.0/0"
    tcp_options { min = 443; max = 443 }
  }
}

# Public Subnet
resource "oci_core_subnet" "public" {
  compartment_id    = oci_identity_compartment.main.id
  vcn_id            = oci_core_vcn.main.id
  cidr_block        = "10.0.0.0/24"
  display_name      = "public-subnet"
  dns_label         = "public"
  route_table_id    = oci_core_route_table.public.id
  security_list_ids = [oci_core_security_list.main.id]
}

# Compute Instance
resource "oci_core_instance" "backend" {
  availability_domain = data.oci_identity_availability_domain.ad.name
  compartment_id      = oci_identity_compartment.main.id
  display_name        = "nomadcrew-backend"
  shape               = "VM.Standard.A1.Flex"

  shape_config {
    ocpus         = 4
    memory_in_gbs = 24
  }

  source_details {
    source_type = "image"
    source_id   = data.oci_core_images.ubuntu_arm.images[0].id
  }

  create_vnic_details {
    subnet_id        = oci_core_subnet.public.id
    assign_public_ip = true
  }

  metadata = {
    ssh_authorized_keys = file(var.ssh_public_key_path)
  }
}

output "instance_public_ip" {
  value = oci_core_instance.backend.public_ip
}
```

### OS-Level Firewall Setup Script (run after SSH)
```bash
#!/bin/bash
# Source: OCI Documentation + community best practices

# Open HTTP port
sudo iptables -I INPUT -p tcp --dport 80 -j ACCEPT

# Open HTTPS port
sudo iptables -I INPUT -p tcp --dport 443 -j ACCEPT

# Open custom app port if needed (e.g., 8080)
# sudo iptables -I INPUT -p tcp --dport 8080 -j ACCEPT

# Make rules persistent
sudo apt-get update
sudo apt-get install -y iptables-persistent
sudo netfilter-persistent save

# Verify rules
sudo iptables -L -n | grep -E "80|443"
```

### OCI CLI Instance Creation (Alternative to Terraform)
```bash
# Source: OCI CLI documentation
# Useful for quick testing or "out of capacity" retry scripts

# Set variables
COMPARTMENT_ID="ocid1.compartment.oc1..xxx"
SUBNET_ID="ocid1.subnet.oc1.iad.xxx"
IMAGE_ID="ocid1.image.oc1.iad.xxx"  # Ubuntu ARM image
SSH_KEY=$(cat ~/.ssh/id_rsa.pub)

# Create instance
oci compute instance launch \
  --compartment-id $COMPARTMENT_ID \
  --availability-domain "Uocm:US-ASHBURN-AD-1" \
  --shape "VM.Standard.A1.Flex" \
  --shape-config '{"ocpus": 4, "memoryInGBs": 24}' \
  --subnet-id $SUBNET_ID \
  --image-id $IMAGE_ID \
  --assign-public-ip true \
  --ssh-authorized-keys "$SSH_KEY" \
  --display-name "nomadcrew-backend"
```
</code_examples>

<sota_updates>
## State of the Art (2025-2026)

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Manual Console setup | Terraform + OCI Provider | Standard since 2022 | Reproducible infrastructure |
| Waiting for capacity | Upgrade to PAYG for priority | 2023-2024 | Eliminates capacity issues without cost |
| Security Lists only | NSGs + Security Lists | 2023+ | NSGs provide finer-grained control |
| AMD micro instances | ARM Ampere A1 Flex | 2021+ | 4 OCPU + 24GB vs 1/8 OCPU + 1GB |

**New tools/patterns to consider:**
- **OCI Resource Manager**: Oracle's managed Terraform service - useful for teams
- **OCI Ampere A4**: Newer ARM generation available (not free tier yet)
- **OCI Bastion Service**: Secure SSH without public IPs (if security is critical)

**Deprecated/outdated:**
- **Manual capacity retry**: Use automated scripts or PAYG upgrade instead
- **Security Lists alone**: NSGs now recommended for application-level rules
- **Free Trial reliance**: Trial expires; Always Free is indefinite
</sota_updates>

<open_questions>
## Open Questions

1. **Region-specific ARM availability**
   - What we know: Some regions better than others (Ashburn, London good; San Jose, Tokyo bad)
   - What's unclear: Real-time capacity status by region
   - Recommendation: Choose Ashburn or London; upgrade to PAYG immediately

2. **Idle reclamation threshold precision**
   - What we know: <20% CPU/Network/Memory over 7 days triggers reclamation
   - What's unclear: Exact measurement methodology (average vs 95th percentile varies in docs)
   - Recommendation: Upgrade to PAYG (free) to eliminate this concern entirely

3. **Coolify ARM compatibility**
   - What we know: Docker works on ARM; Coolify is Docker-based
   - What's unclear: Any ARM-specific issues with Coolify
   - Recommendation: Research further in Phase 14 (Coolify Installation)
</open_questions>

<sources>
## Sources

### Primary (HIGH confidence)
- [Oracle Always Free Resources](https://docs.oracle.com/en-us/iaas/Content/FreeTier/freetier_topic-Always_Free_Resources.htm) - Complete free tier limits and policies
- [OCI Terraform Provider](https://docs.oracle.com/en-us/iaas/Content/dev/terraform/tutorials/tf-provider.htm) - Official Terraform setup
- [OCI Security Rules](https://docs.oracle.com/en-us/iaas/Content/Network/Concepts/securityrules.htm) - Networking security configuration
- [OCI Tenancy Setup Best Practices](https://docs.oracle.com/en-us/iaas/Content/GSG/Concepts/settinguptenancy.htm) - IAM and compartment guidance

### Secondary (MEDIUM confidence - verified with official sources)
- [terraform-oci-free GitHub](https://github.com/frennky/terraform-oci-free) - Complete Terraform module (verified against OCI docs)
- [oci-arm-host-capacity GitHub](https://github.com/hitrov/oci-arm-host-capacity) - Capacity retry script (widely used)
- Community guides on port opening - verified against OCI security documentation

### Tertiary (LOW confidence - needs validation)
- Region availability reports from LowEndTalk/Reddit - anecdotal, varies over time
</sources>

<metadata>
## Metadata

**Research scope:**
- Core technology: Oracle Cloud Infrastructure Always Free tier
- Ecosystem: OCI CLI, Terraform, ARM Ampere A1 compute
- Patterns: VCN networking, Security Lists, compartment organization
- Pitfalls: Capacity issues, dual firewall, idle reclamation, region selection

**Confidence breakdown:**
- Standard stack: HIGH - verified with OCI official documentation
- Architecture: HIGH - from official OCI tutorials and best practices
- Pitfalls: HIGH - documented in official docs + confirmed by community reports
- Code examples: HIGH - from official OCI Terraform provider documentation

**Research date:** 2026-01-10
**Valid until:** 2026-02-10 (30 days - OCI free tier stable, check for policy changes)
</metadata>

---

*Phase: 13-oracle-cloud-setup*
*Research completed: 2026-01-10*
*Ready for planning: yes*
