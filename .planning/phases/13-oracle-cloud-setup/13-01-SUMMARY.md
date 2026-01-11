---
phase: 13-oracle-cloud-setup
plan: 01
subsystem: infra
tags: [aws, ec2, terraform, graviton, arm64, vpc]

# Dependency graph
requires:
  - phase: none
    provides: N/A (first infrastructure phase)
provides:
  - EC2 instance for backend deployment
  - VPC networking with public subnet
  - Security group with ports 22, 80, 443
  - Elastic IP for stable addressing
affects: [14-coolify-installation, 15-cicd-pipeline, 16-application-deployment]

# Tech tracking
tech-stack:
  added: [terraform, aws-provider]
  patterns: [infrastructure-as-code, arm-graviton]

key-files:
  created:
    - infrastructure/aws/main.tf
    - infrastructure/aws/variables.tf
    - infrastructure/aws/outputs.tf
    - infrastructure/aws/terraform.tfvars.example
    - infrastructure/oracle/ (abandoned)
  modified: []

key-decisions:
  - "Switched from Oracle Cloud to AWS due to ARM capacity issues in Dubai region"
  - "Selected t4g.small (ARM Graviton) for cost efficiency (~$14/month)"
  - "Used Elastic IP for stable public addressing"

patterns-established:
  - "Terraform for infrastructure provisioning"
  - "ARM Graviton instances for cost-effective compute"

issues-created: []

# Metrics
duration: 45min
completed: 2026-01-11
---

# Phase 13 Plan 01: Cloud Infrastructure Setup Summary

**AWS EC2 ARM Graviton instance deployed in us-east-2 with VPC, security group, and Elastic IP at 3.130.209.141**

## Performance

- **Duration:** ~45 min (including OCI troubleshooting)
- **Started:** 2026-01-11T01:00:00Z
- **Completed:** 2026-01-11T01:45:00Z
- **Tasks:** 5 (adapted from original plan)
- **Files modified:** 10

## Accomplishments

- AWS EC2 instance running: t4g.small (2 vCPU, 2 GB ARM Graviton)
- VPC networking: 10.0.0.0/16 with public subnet
- Security group: ports 22 (SSH), 80 (HTTP), 443 (HTTPS)
- Elastic IP assigned: 3.130.209.141
- SSH access verified and working
- OCI resources cleaned up (were unusable due to capacity)

## Task Commits

1. **Task 2: OCI Terraform configuration** - `47c0da0` (feat)
2. **Task 4: Firewall script and docs** - `dd4c3ff` (feat)
3. **Task 3: AWS infrastructure deployment** - `ecc08b5` (feat)

## Files Created/Modified

- `infrastructure/aws/main.tf` - VPC, subnet, security group, EC2, EIP
- `infrastructure/aws/variables.tf` - Configuration variables
- `infrastructure/aws/outputs.tf` - Instance IP, SSH command, summary
- `infrastructure/aws/terraform.tfvars.example` - Example config
- `infrastructure/oracle/main.tf` - OCI config (abandoned)
- `infrastructure/oracle/variables.tf` - OCI variables (abandoned)
- `infrastructure/oracle/outputs.tf` - OCI outputs (abandoned)
- `infrastructure/oracle/scripts/setup-firewall.sh` - OS firewall script
- `infrastructure/oracle/README.md` - Infrastructure documentation

## Decisions Made

| Decision | Rationale |
|----------|-----------|
| Switched from OCI to AWS | Oracle Cloud Dubai region had no ARM capacity after 60+ retry attempts |
| t4g.small instance type | Best cost/performance ratio for Go backend + Coolify (~$14/month) |
| ARM Graviton over x86 | 20% cheaper than equivalent x86 instances |
| Elastic IP | Stable IP address that survives instance stop/start |
| us-east-2 (Ohio) | Good latency, reliable capacity, cost-effective |

## Deviations from Plan

### Major Deviation: Cloud Provider Change

- **Original plan:** Oracle Cloud Always Free ARM instance ($0/month)
- **Actual:** AWS EC2 t4g.small (~$14/month)
- **Reason:** Oracle Cloud me-dubai-1 region had persistent "Out of host capacity" errors
- **Attempts:** 60+ automated retries over 1 hour, all failed
- **Impact:** Monthly cost increased from $0 to ~$14, but deployment is reliable

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Windows path expansion**
- **Found during:** Terraform credential configuration
- **Issue:** `~/.oci/` path not expanded on Windows
- **Fix:** Used full Windows path `C:/Users/naqee/.oci/`

## Issues Encountered

- **OCI Capacity:** Dubai region (me-dubai-1) has severe ARM instance shortage
- **Resolution:** Switched to AWS which has reliable capacity

## Next Phase Readiness

Ready for **Phase 14: Coolify Installation**

**Instance Details:**
- IP: `3.130.209.141`
- SSH: `ssh ubuntu@3.130.209.141`
- OS: Ubuntu 22.04 ARM64
- Ports: 22, 80, 443 open

**Prerequisites met:**
- [x] Compute instance running
- [x] SSH access verified
- [x] Ports 80/443 accessible (security group configured)
- [x] Sufficient resources (2 vCPU, 2 GB RAM)

---
*Phase: 13-oracle-cloud-setup*
*Completed: 2026-01-11*
