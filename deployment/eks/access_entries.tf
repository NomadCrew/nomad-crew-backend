# EKS Access Entries for GitHub Actions
# This configuration adds the necessary access entries to allow GitHub Actions to interact with the EKS clusters

# Note: Before applying this configuration, you need to update the authentication mode of your EKS clusters
# to API_AND_CONFIG_MAP using the AWS CLI or Console. This can be done with the following AWS CLI command:
#
# aws eks update-cluster-config \
#   --region us-east-2 \
#   --name nomad-crew-cluster-staging \
#   --authentication-mode API_AND_CONFIG_MAP
#
# aws eks update-cluster-config \
#   --region us-east-2 \
#   --name nomad-crew-cluster-production \
#   --authentication-mode API_AND_CONFIG_MAP

# Access Entry for the GitHub Actions role in Staging
resource "aws_eks_access_entry" "github_actions_staging" {
  cluster_name  = data.aws_eks_cluster.staging.name
  principal_arn = "arn:aws:iam::${var.account_id}:role/NomadCrewStagingDeploymentRole"
  type          = "STANDARD"
}

# Access Entry for the GitHub Actions role in Production
resource "aws_eks_access_entry" "github_actions_production" {
  cluster_name  = data.aws_eks_cluster.production.name
  principal_arn = "arn:aws:iam::${var.account_id}:role/NomadCrewProductionDeploymentRole"
  type          = "STANDARD"
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

# Note: Production access policy association is commented out until the authentication mode is updated
# resource "aws_eks_access_policy_association" "github_actions_production_admin" {
#   cluster_name  = data.aws_eks_cluster.production.name
#   policy_arn    = "arn:aws:eks::aws:cluster-access-policy/AmazonEKSClusterAdminPolicy"
#   principal_arn = aws_eks_access_entry.github_actions_production.principal_arn
#   
#   access_scope {
#     type = "cluster"
#   }
# } 