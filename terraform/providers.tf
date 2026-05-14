# Purpose: configure cloud and Kubernetes providers for the root module.
provider "aws" {
  region = var.region
}

data "aws_availability_zones" "available" {
  state = "available"
}
