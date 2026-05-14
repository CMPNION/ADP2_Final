# Terraform usage

<!-- Purpose: explain how to bootstrap the demo infrastructure from scratch. -->

## What this creates

- a VPC with public and private subnets
- a managed EKS cluster
- app and monitoring namespaces

## How to use

1. Copy the example variables:
   - `cp terraform.tfvars.example terraform.tfvars`
2. Adjust the values for your environment.
3. Initialize Terraform:
   - `terraform init`
4. Validate and plan:
   - `terraform fmt -check`
   - `terraform validate`
   - `terraform plan`

## Environment folders

- `environments/dev` contains dev examples.
- `environments/prod` contains prod examples.

## Notes

- Remote state is expected via the S3 backend in `backend.tf`.
- The Kubernetes provider is wired to the EKS endpoint created by the cluster module.
