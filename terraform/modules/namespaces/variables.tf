# Purpose: list the Kubernetes namespaces created for the platform.
variable "namespaces" {
  type        = list(string)
  description = "Namespaces to create in the cluster."
}
