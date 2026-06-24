output "cluster_name" {
  description = "Kind cluster name (kubectl context: kind-<name>)."
  value       = kind_cluster.arena.name
}

output "gateway_url" {
  description = "Where the in-cluster API answers on the host."
  value       = "http://localhost:${var.gateway_host_port}"
}
