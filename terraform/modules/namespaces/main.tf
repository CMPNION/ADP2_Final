# Purpose: create the app and monitoring namespaces used by the manifests.
resource "kubernetes_namespace_v1" "this" {
  for_each = toset(var.namespaces)

  metadata {
    name = each.value
    labels = {
      "app.kubernetes.io/part-of" = "omnichannel"
    }
  }
}
