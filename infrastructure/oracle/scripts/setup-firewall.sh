#!/bin/bash
# =============================================================================
# NomadCrew - Oracle Cloud Instance Firewall Setup
# =============================================================================
# This script configures iptables on Ubuntu to allow HTTP/HTTPS traffic.
# OCI has TWO firewalls: Security Lists (cloud) + iptables (OS). Both must allow traffic.
#
# Usage:
#   scp setup-firewall.sh ubuntu@<instance-ip>:~
#   ssh ubuntu@<instance-ip> "chmod +x setup-firewall.sh && sudo ./setup-firewall.sh"
# =============================================================================

set -e

echo "=========================================="
echo "NomadCrew Firewall Setup"
echo "=========================================="

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "Please run as root (sudo ./setup-firewall.sh)"
    exit 1
fi

echo ""
echo "[1/4] Opening port 80 (HTTP)..."
iptables -I INPUT -p tcp --dport 80 -j ACCEPT

echo "[2/4] Opening port 443 (HTTPS)..."
iptables -I INPUT -p tcp --dport 443 -j ACCEPT

echo "[3/4] Installing iptables-persistent..."
# Pre-answer the debconf prompts to avoid interactive mode
echo iptables-persistent iptables-persistent/autosave_v4 boolean true | debconf-set-selections
echo iptables-persistent iptables-persistent/autosave_v6 boolean true | debconf-set-selections
DEBIAN_FRONTEND=noninteractive apt-get update -qq
DEBIAN_FRONTEND=noninteractive apt-get install -y -qq iptables-persistent

echo "[4/4] Saving firewall rules..."
netfilter-persistent save

echo ""
echo "=========================================="
echo "Firewall Configuration Complete!"
echo "=========================================="
echo ""
echo "Current iptables rules (ports 80, 443):"
iptables -L INPUT -n --line-numbers | grep -E "dpt:(80|443)" || echo "  (no matching rules found - check manually)"
echo ""
echo "Ports open:"
echo "  - 22  (SSH)   - managed by cloud-init"
echo "  - 80  (HTTP)  - opened by this script"
echo "  - 443 (HTTPS) - opened by this script"
echo ""
echo "Test from your local machine:"
echo "  nc -zv <instance-ip> 80"
echo "  nc -zv <instance-ip> 443"
echo ""
