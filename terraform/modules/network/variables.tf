# Purpose: inputs for the shared VPC module wrapper.
variable "name" {
  type        = string
  description = "VPC name prefix."
}

variable "cidr" {
  type        = string
  description = "CIDR block for the VPC."
}

variable "azs" {
  type        = list(string)
  description = "Availability zones used for public/private subnets."
}

variable "tags" {
  type        = map(string)
  description = "Common resource tags."
  default     = {}
}
