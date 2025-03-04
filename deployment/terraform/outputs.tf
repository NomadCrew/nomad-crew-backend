output "vpc_id" {
  description = "The ID of the VPC"
  value       = module.vpc.vpc_id
}

output "public_subnets" {
  description = "The IDs of the public subnets"
  value       = module.vpc.public_subnets
}

output "private_subnets" {
  description = "The IDs of the private subnets"
  value       = module.vpc.private_subnets
}

output "db_instance_endpoint" {
  description = "The connection endpoint for the RDS instance"
  value       = aws_db_instance.postgres.endpoint
  sensitive   = true
}

output "db_instance_address" {
  description = "The address of the RDS instance"
  value       = aws_db_instance.postgres.address
}

output "db_instance_port" {
  description = "The port of the RDS instance"
  value       = aws_db_instance.postgres.port
}

output "alb_dns_name" {
  description = "The DNS name of the load balancer"
  value       = aws_lb.app_lb.dns_name
}

output "cloudfront_domain_name" {
  description = "The domain name of the CloudFront distribution"
  value       = aws_cloudfront_distribution.app_distribution.domain_name
}

output "s3_bucket_name" {
  description = "The name of the S3 bucket"
  value       = aws_s3_bucket.app_bucket.bucket
}

output "app_url" {
  description = "The URL of the application"
  value       = "https://${var.domain_name}"
}

output "estimated_monthly_cost" {
  description = "Estimated monthly cost for the infrastructure (USD)"
  value       = <<-EOT
    Estimated monthly cost breakdown (USD):
    - EC2 t2.micro (1 instance): Free tier for 12 months, then ~$8.50/month
    - RDS db.t3.micro: Free tier for 12 months, then ~$12.50/month
    - Application Load Balancer: ~$16.50/month
    - NAT Gateway: ~$32.00/month + data processing
    - CloudFront: Free tier includes 1TB data and 10M requests, then variable
    - S3: Free tier includes 5GB storage, then ~$0.023/GB/month
    - Route 53: ~$0.50/month per hosted zone + $0.40/million queries
    - CloudWatch: Free tier includes 10 metrics, then ~$0.30/metric/month
    
    Total estimated cost after free tier: ~$70-100/month
    
    Cost optimization tips:
    1. Use a NAT instance instead of NAT Gateway to save ~$30/month
    2. Consider using a single public subnet and placing the ALB there to eliminate NAT costs
    3. Reduce the number of CloudWatch metrics
    4. Use spot instances for non-critical workloads
  EOT
} 