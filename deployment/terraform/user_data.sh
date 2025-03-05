#!/bin/bash
set -e

# Update system packages
yum update -y
yum install -y amazon-cloudwatch-agent jq git docker awscli

# Start and enable Docker
systemctl start docker
systemctl enable docker

# Install Redis
amazon-linux-extras install -y redis6
systemctl start redis
systemctl enable redis

# Configure Redis
cat > /etc/redis.conf << EOF
bind 127.0.0.1
port 6379
requirepass ${redis_password}
appendonly yes
appendfsync everysec
EOF

# Restart Redis with new configuration
systemctl restart redis

# Create app directory
mkdir -p /opt/nomadcrew

# Create environment file
cat > /opt/nomadcrew/.env << EOF
# Database
DB_HOST=${db_host}
DB_PORT=${db_port}
DB_USER=${db_user}
DB_PASSWORD=${db_password}
DB_NAME=${db_name}
DB_SSL_MODE=require

# Redis
REDIS_ADDRESS=localhost:6379
REDIS_PASSWORD=${redis_password}
REDIS_DB=0

# Server Configuration
SERVER_ENVIRONMENT=${environment}
PORT=${app_port}
ALLOWED_ORIGINS=*
FRONTEND_URL=https://nomadcrew.uk

# AWS Configuration
AWS_REGION=${aws_region}
AWS_SECRETS_PATH=nomadcrew/${environment}/secrets

# Other configurations will be loaded from AWS Secrets Manager
EOF

# Set up CloudWatch agent
cat > /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json << EOF
{
  "agent": {
    "metrics_collection_interval": 60,
    "run_as_user": "root"
  },
  "logs": {
    "logs_collected": {
      "files": {
        "collect_list": [
          {
            "file_path": "/opt/nomadcrew/logs/app.log",
            "log_group_name": "/aws/ec2/nomadcrew",
            "log_stream_name": "{instance_id}/app.log",
            "retention_in_days": 30
          }
        ]
      }
    }
  },
  "metrics": {
    "metrics_collected": {
      "cpu": {
        "measurement": [
          "cpu_usage_idle",
          "cpu_usage_iowait",
          "cpu_usage_user",
          "cpu_usage_system"
        ],
        "metrics_collection_interval": 60,
        "totalcpu": false
      },
      "disk": {
        "measurement": [
          "used_percent",
          "inodes_free"
        ],
        "metrics_collection_interval": 60,
        "resources": [
          "/"
        ]
      },
      "diskio": {
        "measurement": [
          "io_time"
        ],
        "metrics_collection_interval": 60
      },
      "mem": {
        "measurement": [
          "mem_used_percent"
        ],
        "metrics_collection_interval": 60
      },
      "swap": {
        "measurement": [
          "swap_used_percent"
        ],
        "metrics_collection_interval": 60
      }
    },
    "append_dimensions": {
      "AutoScalingGroupName": "$${aws:AutoScalingGroupName}",
      "ImageId": "$${aws:ImageId}",
      "InstanceId": "$${aws:InstanceId}",
      "InstanceType": "$${aws:InstanceType}"
    }
  }
}
EOF

# Start CloudWatch agent
systemctl start amazon-cloudwatch-agent
systemctl enable amazon-cloudwatch-agent

# Create logs directory
mkdir -p /opt/nomadcrew/logs

# Login to ECR
aws ecr get-login-password --region ${aws_region} | docker login --username AWS --password-stdin ${ecr_repository_uri}

# Pull the Docker image
docker pull ${ecr_repository_uri}:${image_tag}

# Create Docker compose file
cat > /opt/nomadcrew/docker-compose.yml << EOF
version: '3'

services:
  api:
    image: ${ecr_repository_uri}:${image_tag}
    restart: always
    ports:
      - "${app_port}:${app_port}"
    env_file:
      - .env
    volumes:
      - ./logs:/app/logs
    depends_on:
      - redis
    networks:
      - app-network

  redis:
    image: redis:latest
    command: redis-server --requirepass ${redis_password}
    volumes:
      - redis-data:/data
    networks:
      - app-network

networks:
  app-network:
    driver: bridge

volumes:
  redis-data:
EOF

# Start the application with Docker Compose
cd /opt/nomadcrew
docker-compose up -d

# Set up log rotation
cat > /etc/logrotate.d/nomadcrew << EOF
/opt/nomadcrew/logs/app.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
    create 644 root root
    postrotate
        cd /opt/nomadcrew && docker-compose restart api
    endscript
}
EOF

echo "Instance setup completed" 