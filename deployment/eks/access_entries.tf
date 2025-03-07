# EKS Access Entries for GitHub Actions
# This configuration adds the necessary access entries to allow GitHub Actions to interact with the EKS clusters

# Update authentication mode for staging cluster
resource "aws_eks_cluster_auth_config" "staging_auth_config" {
  name = data.aws_eks_cluster.staging.name
  authentication_mode = "API_AND_CONFIG_MAP"
}

# Update authentication mode for production cluster
resource "aws_eks_cluster_auth_config" "production_auth_config" {
  name = data.aws_eks_cluster.production.name
  authentication_mode = "API_AND_CONFIG_MAP"
}

# Access Entry for the GitHub Actions role in Staging
resource "aws_eks_access_entry" "github_actions_staging" {
  cluster_name  = data.aws_eks_cluster.staging.name
  principal_arn = "arn:aws:iam::${var.account_id}:role/NomadCrewStagingDeploymentRole"
  type          = "STANDARD"
  
  depends_on = [
    aws_eks_cluster_auth_config.staging_auth_config
  ]
}

# Access Entry for the GitHub Actions role in Production
resource "aws_eks_access_entry" "github_actions_production" {
  cluster_name  = data.aws_eks_cluster.production.name
  principal_arn = "arn:aws:iam::${var.account_id}:role/NomadCrewProductionDeploymentRole"
  type          = "STANDARD"
  
  depends_on = [
    aws_eks_cluster_auth_config.production_auth_config
  ]
}

# Access Policy Association for Staging
resource "aws_eks_access_policy_association" "github_actions_staging_admin" {
  cluster_name  = data.aws_eks_cluster.staging.name
  policy_arn    = "arn:aws:eks::aws:cluster-access-policy/AmazonEKSClusterAdminPolicy"
  principal_arn = aws_eks_access_entry.github_actions_staging.principal_arn
  
  access_scope {
    type = "cluster"
  }
}

# Access Policy Association for Production
resource "aws_eks_access_policy_association" "github_actions_production_admin" {
  cluster_name  = data.aws_eks_cluster.production.name
  policy_arn    = "arn:aws:eks::aws:cluster-access-policy/AmazonEKSClusterAdminPolicy"
  principal_arn = aws_eks_access_entry.github_actions_production.principal_arn
  
  access_scope {
    type = "cluster"
  }
} 