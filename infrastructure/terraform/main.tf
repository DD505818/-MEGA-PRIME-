terraform {
  required_version = ">= 1.6.0"
}
provider "kubernetes" {
  config_path = "~/.kube/config"
}
