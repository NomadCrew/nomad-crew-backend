provider "aws" {
  region = "us-east-2"
}

# Variables
variable "account_id" {
  default = "195275657852"
}

variable "subnet_ids" {
  description = "Subnet IDs for EKS"
  type        = list(string)
  default     = ["subnet-07a5881361a05010c", "subnet-0007e059a87a89f20", "subnet-05ad85e775b2c83d4"]
}

variable "vpc_id" {
  description = "VPC ID for EKS"
  type        = string
  default     = "vpc-03056898e3498bb97" # This was identified from your previous output
}

# IAM Roles
resource "aws_iam_role" "eks_cluster_role" {
  name = "EksClusterRole"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Service = "eks.amazonaws.com"
        }
        Action = "sts:AssumeRole"
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "eks_cluster_policy" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"
  role       = aws_iam_role.eks_cluster_role.name
}

resource "aws_iam_role" "eks_node_role" {
  name = "EksNodeRole"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          Service = "ec2.amazonaws.com"
        }
        Action = "sts:AssumeRole"
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "eks_worker_node_policy" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy"
  role       = aws_iam_role.eks_node_role.name
}

resource "aws_iam_role_policy_attachment" "eks_cni_policy" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy"
  role       = aws_iam_role.eks_node_role.name
}

resource "aws_iam_role_policy_attachment" "ecr_read_only" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
  role       = aws_iam_role.eks_node_role.name
}

# EKS Clusters - Using data sources instead of resources since clusters already exist
data "aws_eks_cluster" "staging" {
  name = "nomad-crew-cluster-staging"
}

data "aws_eks_cluster" "production" {
  name = "nomad-crew-cluster-production"
}

# Comment out the resource blocks for existing clusters
/*
resource "aws_eks_cluster" "staging" {
  name     = "nomad-crew-cluster-staging"
  role_arn = aws_iam_role.eks_cluster_role.arn
  version  = "1.28"

  vpc_config {
    subnet_ids              = var.subnet_ids
    endpoint_private_access = false
    endpoint_public_access  = true
  }

  depends_on = [
    aws_iam_role_policy_attachment.eks_cluster_policy
  ]
}

resource "aws_eks_cluster" "production" {
  name     = "nomad-crew-cluster-production"
  role_arn = aws_iam_role.eks_cluster_role.arn
  version  = "1.28"

  vpc_config {
    subnet_ids              = var.subnet_ids
    endpoint_private_access = true
    endpoint_public_access  = true
  }

  depends_on = [
    aws_iam_role_policy_attachment.eks_cluster_policy
  ]
}
*/

# EKS Node Groups
resource "aws_eks_node_group" "staging_nodes" {
  cluster_name    = data.aws_eks_cluster.staging.name
  node_group_name = "staging-workers"
  node_role_arn   = aws_iam_role.eks_node_role.arn
  subnet_ids      = [var.subnet_ids[0], var.subnet_ids[1]]
  instance_types  = ["t3a.small"]
  capacity_type   = "SPOT"
  disk_size       = 20

  scaling_config {
    desired_size = 1
    max_size     = 2
    min_size     = 1
  }

  depends_on = [
    aws_iam_role_policy_attachment.eks_worker_node_policy,
    aws_iam_role_policy_attachment.eks_cni_policy,
    aws_iam_role_policy_attachment.ecr_read_only,
  ]
}

resource "aws_eks_node_group" "production_nodes" {
  cluster_name    = data.aws_eks_cluster.production.name
  node_group_name = "prod-workers"
  node_role_arn   = aws_iam_role.eks_node_role.arn
  subnet_ids      = var.subnet_ids
  instance_types  = ["t3a.medium"]
  capacity_type   = "ON_DEMAND"
  disk_size       = 50

  scaling_config {
    desired_size = 3
    max_size     = 6
    min_size     = 3
  }

  depends_on = [
    aws_iam_role_policy_attachment.eks_worker_node_policy,
    aws_iam_role_policy_attachment.eks_cni_policy,
    aws_iam_role_policy_attachment.ecr_read_only,
  ]
}

# Outputs
output "staging_cluster_endpoint" {
  description = "Endpoint for EKS staging control plane"
  value       = data.aws_eks_cluster.staging.endpoint
}

output "production_cluster_endpoint" {
  description = "Endpoint for EKS production control plane"
  value       = data.aws_eks_cluster.production.endpoint
}

output "staging_kubeconfig_command" {
  description = "Command to configure kubectl for staging"
  value       = "aws eks update-kubeconfig --region us-east-2 --name ${data.aws_eks_cluster.staging.name}"
}

output "production_kubeconfig_command" {
  description = "Command to configure kubectl for production"
  value       = "aws eks update-kubeconfig --region us-east-2 --name ${data.aws_eks_cluster.production.name}"
}