# Purpose: surface the key endpoints needed for kubeconfig, ingress, and demos.
output "cluster_name" {
  value       = module.cluster.cluster_name
  description = "Managed Kubernetes cluster name."
}

output "cluster_endpoint" {
  value       = module.cluster.cluster_endpoint
  description = "Managed Kubernetes API endpoint."
}

output "vpc_id" {
  value       = module.network.vpc_id
  description = "VPC identifier used by the cluster."
}

output "namespaces" {
  value       = module.namespaces.namespaces
  description = "Namespaces created for the platform."
}
