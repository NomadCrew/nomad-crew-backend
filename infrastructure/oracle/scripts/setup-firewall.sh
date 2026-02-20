#!/bin/bash
# =============================================================================
# NomadCrew - Oracle Cloud Instance Firewall & SSH Hardening
# =============================================================================
# This script configures iptables on Ubuntu to allow HTTP/HTTPS traffic
# and hardens SSH configuration.
# OCI has TWO firewalls: Security Lists (cloud) + iptables (OS). Both must allow traffic.
#
# Port 8000 (Coolify dashboard) is intentionally NOT opened here.
# Access Coolify via SSH tunnel: ssh -L 8000:localhost:8000 ubuntu@<instance-ip>
# Then open http://localhost:8000 in your browser.
#
# Usage:
#   scp setup-firewall.sh ubuntu@<instance-ip>:~
#   ssh ubuntu@<instance-ip> "chmod +x setup-firewall.sh && sudo ./setup-firewall.sh"
# =============================================================================

set -e

echo "=========================================="
echo "NomadCrew Firewall & SSH Hardening"
echo "=========================================="

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "Please run as root (sudo ./setup-firewall.sh)"
    exit 1
fi

echo ""
echo "[1/7] Opening port 80 (HTTP)..."
iptables -I INPUT -p tcp --dport 80 -j ACCEPT

echo "[2/7] Opening port 443 (HTTPS)..."
iptables -I INPUT -p tcp --dport 443 -j ACCEPT

echo "[3/7] Installing iptables-persistent..."
# Pre-answer the debconf prompts to avoid interactive mode
echo iptables-persistent iptables-persistent/autosave_v4 boolean true | debconf-set-selections
echo iptables-persistent iptables-persistent/autosave_v6 boolean true | debconf-set-selections
DEBIAN_FRONTEND=noninteractive apt-get update -qq
DEBIAN_FRONTEND=noninteractive apt-get install -y -qq iptables-persistent

echo "[4/7] Saving firewall rules..."
netfilter-persistent save

echo "[5/7] Hardening SSH configuration..."
sed -i 's/#\?PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config
sed -i 's/#\?PermitRootLogin.*/PermitRootLogin no/' /etc/ssh/sshd_config
sed -i 's/#\?MaxAuthTries.*/MaxAuthTries 3/' /etc/ssh/sshd_config
sed -i 's/#\?X11Forwarding.*/X11Forwarding no/' /etc/ssh/sshd_config
systemctl restart sshd

echo "[6/7] Installing fail2ban..."
DEBIAN_FRONTEND=noninteractive apt-get install -y -qq fail2ban
# Configure fail2ban for SSH
cat > /etc/fail2ban/jail.local << 'JAILEOF'
[sshd]
enabled = true
port = ssh
filter = sshd
logpath = /var/log/auth.log
maxretry = 3
bantime = 3600
findtime = 600
JAILEOF
systemctl enable fail2ban
systemctl restart fail2ban

echo "[7/7] Installing unattended-upgrades..."
DEBIAN_FRONTEND=noninteractive apt-get install -y -qq unattended-upgrades
# Enable automatic security updates
cat > /etc/apt/apt.conf.d/20auto-upgrades << 'UPGRADEEOF'
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
APT::Periodic::AutocleanInterval "7";
UPGRADEEOF

echo ""
echo "=========================================="
echo "Setup Complete!"
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
echo "SSH hardening applied:"
echo "  - PasswordAuthentication no"
echo "  - PermitRootLogin no"
echo "  - MaxAuthTries 3"
echo "  - fail2ban active (3 retries, 1h ban)"
echo ""
echo "Automatic security updates: enabled"
echo ""
echo "Coolify dashboard: access via SSH tunnel only"
echo "  ssh -L 8000:localhost:8000 ubuntu@<instance-ip>"
echo ""
