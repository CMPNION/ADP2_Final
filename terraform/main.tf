# Purpose: assemble the network, EKS cluster, and namespace layers.
locals {
  cluster_name = "${var.project_name}-${var.environment}"
  azs          = slice(data.aws_availability_zones.available.names, 0, 2)
}

module "network" {
  source = "./modules/network"

  name = local.cluster_name
  cidr = var.vpc_cidr
  azs  = local.azs
  tags = { Project = var.project_name, Environment = var.environment }
}

module "cluster" {
  source = "./modules/cluster"

  name                = local.cluster_name
  cluster_version     = var.cluster_version
  vpc_id              = module.network.vpc_id
  private_subnets     = module.network.private_subnets
  node_instance_types = var.node_instance_types
  node_desired_size   = var.node_desired_size
  node_min_size       = var.node_min_size
  node_max_size       = var.node_max_size
  tags                = { Project = var.project_name, Environment = var.environment }
}

data "aws_eks_cluster" "this" {
  name = module.cluster.cluster_name
}

data "aws_eks_cluster_auth" "this" {
  name = module.cluster.cluster_name
}

provider "kubernetes" {
  host                   = data.aws_eks_cluster.this.endpoint
  cluster_ca_certificate = base64decode(data.aws_eks_cluster.this.certificate_authority[0].data)
  token                  = data.aws_eks_cluster_auth.this.token
}

module "namespaces" {
  source = "./modules/namespaces"

  namespaces = var.namespaces
  depends_on = [module.cluster]
}
