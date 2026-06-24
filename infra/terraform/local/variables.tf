variable "cluster_name" {
  description = "Name of the Kind cluster. Distinct from the task-managed 'arena' cluster so the two workflows never fight over state."
  type        = string
  default     = "arena-tf"
}

variable "gateway_host_port" {
  description = "Host port mapped to the gateway NodePort (30080)."
  type        = number
  default     = 8091
}

variable "image_tag" {
  description = "Tag of the locally built arena-gateway / arena-executor images."
  type        = string
  default     = "dev"
}
