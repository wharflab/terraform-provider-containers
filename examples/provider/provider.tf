terraform {
  required_providers {
    containers = {
      source  = "wharflab/containers"
      version = "~> 0.1"
    }
  }
}

provider "containers" {
  region = "eu-west-1"
}
