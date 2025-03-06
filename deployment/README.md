# NomadCrew AWS Infrastructure

This directory contains the Terraform and Terragrunt configurations for deploying the NomadCrew backend application to AWS.

## Architecture Overview

The infrastructure is designed to be cost-effective while supporting 1000 active users globally. It maximizes the use of AWS Free Tier resources where possible while ensuring high availability and security.

### Key Components

1. **Compute**: EC2 t2.micro instances (free tier eligible) in an Auto Scaling Group
2. **Database**: RDS PostgreSQL db.t3.micro (free tier eligible) with optional read replica
3. **Caching**: ElastiCache Redis with replication for high availability
4. **Storage**: S3 for ALB logs and static assets
5. **Load Balancing**: Application Load Balancer (ALB) with WAF protection
6. **Monitoring**: CloudWatch basic monitoring with alarms
7. **Security**: Security Groups, IAM, AWS Certificate Manager, WAF
8. **Networking**: VPC with public and private subnets, redundant NAT Instances (t3.nano)
9. **Container Registry**: ECR for Docker images

### Architecture Diagram

```
                                  ┌─────────────────┐
                                  │   Route 53      │
                                  │   (DNS)         │
                                  └────────┬────────┘
                                           │
                                           ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      Application Load Balancer                       │
│                           with WAF protection                        │
└─────────────────────────────────┬───────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        Auto Scaling Group                            │
│                                                                      │
│    ┌─────────────────┐                      ┌─────────────────┐     │
│    │  EC2 Instance   │                      │  EC2 Instance   │     │
│    │                 │                      │                 │     │
│    │  ┌───────────┐  │                      │  ┌───────────┐  │     │
│    │  │ Docker    │  │                      │  │ Docker    │  │     │
│    │  │ Container │  │                      │  │ Container │  │     │
│    │  └───────────┘  │                      │  └───────────┘  │     │
│    │                 │                      │                 │     │
│    └─────────────────┘                      └─────────────────┘     │
│                                                                      │
└─────────────────────────────────┬───────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         RDS PostgreSQL                               │
│                      (with optional read replica)                    │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│                         ElastiCache Redis                            │
│                      (with replication for HA)                       │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│                         NAT Instances (t3.nano)                      │
│                      (one per AZ for redundancy)                     │
└─────────────────────────────────────────────────────────────────────┘
```

## Cost Optimization

This architecture is designed to maximize AWS Free Tier usage:

- **EC2**: t2.micro instances (750 hours/month free for 12 months)
- **RDS**: db.t3.micro (750 hours/month free for 12 months)
- **NAT Instances**: t3.nano instead of NAT Gateway (~$32/month savings)
- **S3**: 5GB storage, 20,000 GET, 2,000 PUT requests free
- **CloudWatch**: Basic monitoring free
- **ElastiCache**: Not free tier eligible, but t3.micro instances are cost-effective

Estimated monthly cost after free tier: ~$50-70/month (compared to ~$70-100/month with NAT Gateway)

## Security Features

The infrastructure includes several security features:

1. **AWS WAF**: Protects against common web exploits and DDoS attacks
2. **Security Groups**: Restricts access to resources based on least privilege
3. **SSH Access**: Limited to specific IP ranges
4. **Encryption**: All data encrypted at rest and in transit
5. **IAM Roles**: Follow principle of least privilege
6. **SSL/TLS**: For all public endpoints

## High Availability

The infrastructure is designed for high availability:

1. **Multiple AZs**: Resources deployed across multiple availability zones
2. **Redundant NAT Instances**: One per AZ for network redundancy
3. **Auto Scaling**: Automatically scales based on load
4. **ElastiCache Replication**: Redis with automatic failover
5. **RDS Read Replica**: Optional read replica for read-heavy workloads (enabled in production)

## Deployment

### Prerequisites

1. AWS CLI configured with appropriate credentials
2. Terraform (v1.5.7+)
3. Terragrunt (v0.51.1+)
4. Docker (for local testing)

### Initial Setup

1. Create an S3 bucket for Terraform state:

```bash
aws s3api create-bucket --bucket nomadcrew-terraform-state --region us-east-2
aws s3api put-bucket-versioning --bucket nomadcrew-terraform-state --versioning-configuration Status=Enabled
```

2. Create a DynamoDB table for state locking:

```bash
aws dynamodb create-table \
  --table-name nomadcrew-terraform-locks \
  --attribute-definitions AttributeName=LockID,AttributeType=S \
  --key-schema AttributeName=LockID,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST \
  --region us-east-2
```

3. Create an ECR repository:

```bash
aws ecr create-repository --repository-name nomadcrew --region us-east-2
```

### Manual Deployment

1. Set required environment variables:

```bash
export TF_VAR_db_password="your-secure-password"
export TF_VAR_redis_password="your-secure-redis-password"
export TF_VAR_route53_zone_id="your-zone-id"
export TF_VAR_certificate_arn="your-certificate-arn"
export TF_VAR_key_name="your-ssh-key-name"
export TF_VAR_ssh_allowed_cidrs='["10.0.0.0/24", "192.168.1.0/24"]'  # Replace with your allowed CIDR blocks
```

2. Deploy using Terragrunt:

```bash
cd deployment/environments/dev  # or prod
terragrunt init
terragrunt plan
terragrunt apply
```

### AWS Copilot Deployment

For simplified deployment, you can use AWS Copilot:

1. Install AWS Copilot CLI:
   ```
   sudo curl -Lo /usr/local/bin/copilot https://github.com/aws/copilot-cli/releases/latest/download/copilot-linux && sudo chmod +x /usr/local/bin/copilot
   ```

2. Initialize Copilot in your project directory:
   ```
   copilot init
   ```

3. Follow the prompts to set up your application and service

4. Deploy your service:
   ```
   copilot svc deploy --name nomadcrew-backend
   ```

## Environment Management

The infrastructure is organized to support multiple environments:

- **Dev**: `deployment/environments/dev`
- **Prod**: `deployment/environments/prod`

Each environment can have its own configuration while sharing common modules.

## Secrets Management

Sensitive information is managed through:

1. GitHub Secrets for CI/CD
2. AWS Secrets Manager for application secrets
3. Environment variables for local development

## Monitoring and Logging

- CloudWatch for metrics and logs
- CloudWatch Alarms for auto-scaling and notifications
- AWS Budgets for cost monitoring

## Security Considerations

- All data is encrypted at rest and in transit
- Security groups restrict access to resources
- IAM roles follow the principle of least privilege
- SSL/TLS for all public endpoints

## Troubleshooting

### Common Issues

1. **Deployment Failures**:
   - Check CloudWatch Logs
   - Verify IAM permissions
   - Ensure S3 bucket and DynamoDB table exist

2. **Application Health Issues**:
   - Check instance logs: `/opt/nomadcrew/logs/app.log`
   - Verify database connectivity
   - Check security group rules

3. **Performance Issues**:
   - Monitor CloudWatch metrics
   - Consider scaling up resources if consistently high utilization 

4. **NAT Instance Issues**:
   - Verify source/destination check is disabled
   - Check security group rules
   - Ensure proper route table configuration

## Next Steps and Future Considerations

1. Collect and analyze usage metrics after 2-4 weeks of production data
2. Consider migrating Redis to Amazon ElastiCache for better management
3. Evaluate need for read replicas in RDS as traffic grows
4. Plan for potential migration to container-based deployment (ECS/EKS) for better resource utilization 