# NomadCrew AWS Infrastructure

This directory contains the Terraform and Terragrunt configurations for deploying the NomadCrew backend application to AWS.

## Architecture Overview

The infrastructure is designed to be cost-effective while supporting 1000 active users globally. It maximizes the use of AWS Free Tier resources where possible.

### Key Components

1. **Compute**: EC2 t2.micro instances (free tier eligible) in an Auto Scaling Group
2. **Database**: RDS PostgreSQL db.t3.micro (free tier eligible)
3. **Caching**: Self-hosted Redis on EC2 instances
4. **Storage**: S3 for backups and static assets
5. **CDN**: CloudFront for global content delivery
6. **Monitoring**: CloudWatch basic monitoring
7. **Security**: Security Groups, IAM, AWS Certificate Manager
8. **Networking**: VPC with public and private subnets, NAT Gateway
9. **Container Registry**: ECR for Docker images

### Architecture Diagram

```
                                  ┌─────────────────┐
                                  │   CloudFront    │
                                  │   Distribution  │
                                  └────────┬────────┘
                                           │
                                           ▼
┌─────────────────────────────────────────────────────────────────────┐
│                              Route 53                                │
└─────────────────────────────────┬───────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      Application Load Balancer                       │
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
│    │  ┌───────────┐  │                      │  ┌───────────┐  │     │
│    │  │  Redis    │  │                      │  │  Redis    │  │     │
│    │  └───────────┘  │                      │  └───────────┘  │     │
│    └─────────────────┘                      └─────────────────┘     │
│                                                                      │
└─────────────────────────────────┬───────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         RDS PostgreSQL                               │
└─────────────────────────────────────────────────────────────────────┘
```

## Cost Optimization

This architecture is designed to maximize AWS Free Tier usage:

- **EC2**: t2.micro instances (750 hours/month free for 12 months)
- **RDS**: db.t3.micro (750 hours/month free for 12 months)
- **S3**: 5GB storage, 20,000 GET, 2,000 PUT requests free
- **CloudFront**: 1TB data transfer out, 10M requests free
- **CloudWatch**: Basic monitoring free

Estimated monthly cost after free tier: ~$70-100/month

## Deployment

### Prerequisites

1. AWS CLI configured with appropriate credentials
2. Terraform (v1.5.7+)
3. Terragrunt (v0.51.1+)
4. Docker (for local testing)

### Initial Setup

1. Create an S3 bucket for Terraform state:

```bash
aws s3api create-bucket --bucket nomadcrew-terraform-state --region us-east-1
aws s3api put-bucket-versioning --bucket nomadcrew-terraform-state --versioning-configuration Status=Enabled
```

2. Create a DynamoDB table for state locking:

```bash
aws dynamodb create-table \
  --table-name nomadcrew-terraform-locks \
  --attribute-definitions AttributeName=LockID,AttributeType=S \
  --key-schema AttributeName=LockID,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST \
  --region us-east-1
```

3. Create an ECR repository:

```bash
aws ecr create-repository --repository-name nomadcrew --region us-east-1
```

### Manual Deployment

1. Set required environment variables:

```bash
export TF_VAR_db_password="your-secure-password"
export TF_VAR_route53_zone_id="your-zone-id"
export TF_VAR_certificate_arn="your-certificate-arn"
export TF_VAR_key_name="your-ssh-key-name"
```

2. Deploy using Terragrunt:

```bash
cd deployment/environments/dev  # or prod
terragrunt init
terragrunt plan
terragrunt apply
```

### CI/CD Deployment

The project includes a GitHub Actions workflow that automatically:

1. Builds and pushes a Docker image to ECR
2. Deploys the infrastructure using Terragrunt
3. Updates the application with the new image
4. Verifies the deployment

The workflow is triggered on pushes to the main branch.

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