# EKS Access Entries for GitHub Actions
# This configuration adds the necessary access entries to allow GitHub Actions to interact with the EKS clusters

# Access Entry for the GitHub Actions role in Staging
resource "aws_eks_access_entry" "github_actions_staging" {
  cluster_name      = aws_eks_cluster.staging.name
  principal_arn     = "arn:aws:iam::${var.account_id}:role/NomadCrewStagingDeploymentRole"
  type              = "STANDARD"
  kubernetes_groups = ["system:masters"]
}

# Access Entry for the GitHub Actions role in Production
resource "aws_eks_access_entry" "github_actions_production" {
  cluster_name      = aws_eks_cluster.production.name
  principal_arn     = "arn:aws:iam::${var.account_id}:role/NomadCrewProductionDeploymentRole"
  type              = "STANDARD"
  kubernetes_groups = ["system:masters"]
}

# Access Policy Association for Staging
resource "aws_eks_access_policy_association" "github_actions_staging_admin" {
  cluster_name  = aws_eks_cluster.staging.name
  policy_arn    = "arn:aws:eks::aws:cluster-access-policy/AmazonEKSClusterAdminPolicy"
  principal_arn = "arn:aws:iam::${var.account_id}:role/NomadCrewStagingDeploymentRole"
  access_scope {
    type       = "cluster"
  }
}

# Access Policy Association for Production
resource "aws_eks_access_policy_association" "github_actions_production_admin" {
  cluster_name  = aws_eks_cluster.production.name
  policy_arn    = "arn:aws:eks::aws:cluster-access-policy/AmazonEKSClusterAdminPolicy"
  principal_arn = "arn:aws:iam::${var.account_id}:role/NomadCrewProductionDeploymentRole"
  access_scope {
    type       = "cluster"
  }
} 