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
  sensitive   = false
}

output "db_instance_port" {
  description = "The port of the RDS instance"
  value       = aws_db_instance.postgres.port
}

output "db_instance_name" {
  description = "The database name"
  value       = aws_db_instance.postgres.db_name
}

output "db_instance_username" {
  description = "The master username for the database"
  value       = aws_db_instance.postgres.username
  sensitive   = true
}

output "alb_dns_name" {
  description = "The DNS name of the load balancer"
  value       = aws_lb.app_lb.dns_name
}

output "alb_zone_id" {
  description = "The canonical hosted zone ID of the load balancer"
  value       = aws_lb.app_lb.zone_id
}

output "ecr_repository_url" {
  description = "The URL of the ECR repository"
  value       = aws_ecr_repository.app.repository_url
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

output "nat_instance_id" {
  description = "The ID of the NAT instance"
  value       = aws_instance.nat_instance.id
}

output "nat_instance_public_ip" {
  description = "The public IP of the NAT instance"
  value       = aws_instance.nat_instance.public_ip
}

output "redis_instance_id" {
  description = "The ID of the Redis instance"
  value       = aws_instance.redis.id
}

output "redis_instance_private_ip" {
  description = "The private IP of the Redis instance"
  value       = aws_instance.redis.private_ip
}

output "alb_logs_bucket" {
  description = "The name of the S3 bucket for ALB logs"
  value       = aws_s3_bucket.alb_logs.id
}

output "cloudwatch_alarms" {
  description = "The names of the CloudWatch alarms"
  value = [
    aws_cloudwatch_metric_alarm.cpu_alarm.alarm_name,
    aws_cloudwatch_metric_alarm.db_cpu_alarm.alarm_name,
    aws_cloudwatch_metric_alarm.db_storage_alarm.alarm_name,
    aws_cloudwatch_metric_alarm.alb_5xx_alarm.alarm_name
  ]
} 