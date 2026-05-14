# Purpose: inputs for the shared EKS module wrapper.
variable "name" {
  type        = string
  description = "Cluster name."
}

variable "cluster_version" {
  type        = string
  description = "Kubernetes version."
}

variable "vpc_id" {
  type        = string
  description = "VPC identifier."
}

variable "private_subnets" {
  type        = list(string)
  description = "Private subnet IDs for the worker nodes."
}

variable "node_instance_types" {
  type        = list(string)
  description = "Worker node instance types."
}

variable "node_desired_size" {
  type        = number
  description = "Desired node count."
}

variable "node_min_size" {
  type        = number
  description = "Minimum node count."
}

variable "node_max_size" {
  type        = number
  description = "Maximum node count."
}

variable "tags" {
  type        = map(string)
  description = "Common resource tags."
  default     = {}
}
