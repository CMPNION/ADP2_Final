# Purpose: wrap the official VPC module with platform-specific defaults.
module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "~> 5.0"

  name = var.name
  cidr = var.cidr

  azs             = var.azs
  private_subnets = [for idx, az in var.azs : cidrsubnet(var.cidr, 4, idx + 1)]
  public_subnets  = [for idx, az in var.azs : cidrsubnet(var.cidr, 4, idx + 10)]

  enable_nat_gateway   = true
  single_nat_gateway   = true
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = var.tags
}
