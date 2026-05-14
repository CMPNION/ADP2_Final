# Purpose: shared Terraform inputs for the cluster and namespace stack.
variable "project_name" {
  description = "Project prefix used for all cloud resources."
  type        = string
  default     = "omnichannel"
}

variable "environment" {
  description = "Deployment environment name (dev or prod)."
  type        = string
}

variable "region" {
  description = "AWS region used for the demo cluster."
  type        = string
  default     = "eu-central-1"
}

variable "vpc_cidr" {
  description = "CIDR block for the platform VPC."
  type        = string
  default     = "10.30.0.0/16"
}

variable "cluster_version" {
  description = "Kubernetes version for the managed EKS cluster."
  type        = string
  default     = "1.30"
}

variable "node_instance_types" {
  description = "Instance types used for the worker node group."
  type        = list(string)
  default     = ["t3.medium"]
}

variable "node_desired_size" {
  description = "Desired worker node count."
  type        = number
  default     = 2
}

variable "node_min_size" {
  description = "Minimum worker node count."
  type        = number
  default     = 2
}

variable "node_max_size" {
  description = "Maximum worker node count."
  type        = number
  default     = 5
}

variable "namespaces" {
  description = "Namespaces created for the application and monitoring stack."
  type        = list(string)
  default     = ["omnichannel", "monitoring", "ingress-nginx"]
}
