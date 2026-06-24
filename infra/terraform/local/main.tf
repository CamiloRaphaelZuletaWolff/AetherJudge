# Local IaC: provisions the Kind cluster and the arena Helm release from
# nothing, on the developer machine. Deliberately targets the environment
# that actually exists (no cloud account) — the module boundary (variables
# for tags/ports, values overrides) is the seam a cloud root module would
# reuse with managed PostgreSQL/Redis and a registry. See README.md.

terraform {
  required_version = ">= 1.7"

  required_providers {
    kind = {
      source  = "tehcyx/kind"
      version = "~> 0.9"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "~> 3.0"
    }
    null = {
      source  = "hashicorp/null"
      version = "~> 3.2"
    }
  }
}

provider "kind" {}

resource "kind_cluster" "arena" {
  name           = var.cluster_name
  wait_for_ready = true

  kind_config {
    kind        = "Cluster"
    api_version = "kind.x-k8s.io/v1alpha4"

    node {
      role = "control-plane"

      extra_port_mappings {
        container_port = 30080
        host_port      = var.gateway_host_port
      }
    }
  }
}

# Kind has no registry; service images are side-loaded from the host Docker
# daemon. Build them first: task k8s:images (task tf:apply does this).
resource "null_resource" "load_images" {
  depends_on = [kind_cluster.arena]

  triggers = {
    cluster   = kind_cluster.arena.id
    image_tag = var.image_tag
  }

  provisioner "local-exec" {
    # Service images plus every dependency image the host already has —
    # side-loading beats re-pulling hundreds of MB through slow networks.
    command = "kind load docker-image arena-gateway:${var.image_tag} arena-executor:${var.image_tag} postgres:16-alpine redis:7-alpine docker:28-dind docker:28-cli --name ${var.cluster_name}"
  }
}

provider "helm" {
  kubernetes = {
    host                   = kind_cluster.arena.endpoint
    client_certificate     = kind_cluster.arena.client_certificate
    client_key             = kind_cluster.arena.client_key
    cluster_ca_certificate = kind_cluster.arena.cluster_ca_certificate
  }
}

resource "helm_release" "arena" {
  depends_on = [null_resource.load_images]

  name  = "arena"
  chart = "${path.module}/../../helm/arena"
  wait  = true
  # Generous: first start builds sandbox images inside DinD, and that cost
  # is network-bound (the in-DinD base pulls can't be side-loaded).
  timeout = 1800

  set = [
    {
      name  = "gateway.tag"
      value = var.image_tag
    },
    {
      name  = "executor.tag"
      value = var.image_tag
    },
  ]
}
