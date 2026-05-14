# Purpose: expose the created namespace names for root outputs.
output "namespaces" {
  value = [for ns in kubernetes_namespace_v1.this : ns.metadata[0].name]
}
