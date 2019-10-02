//-------------------------------------------------------------------
// AWS settings
//-------------------------------------------------------------------
//create an ami??
variable "ami" {
    default = "ami-c59ce2bd"
    description = "AMI for Vault instances"
}

variable "availability-zones" {
    default = "us-west-2a"
    description = "Availability zones for launching the Vault instances"
}

variable "instance_type" {
    default = "t2.micro"
    description = "Instance type for Vault instances"
}

variable "key-name" {
    default = "deploy"
    description = "SSH key name for Vault instances"
}
//get rid of one of these - not valid (figure out which one)
variable "subnets" {
    description = "list of subnets to launch Vault within"
}

variable "ec2role" {
    description = "Ec2 Role"
}

// example: export TF_VAR_security_group_names='["sg-example-one","sg-example-two"]'
variable "security_group_names" {
  type    = list(string)
  default = ["sg-dc92adb8","sg-5c505b38","sg-306a034b","sg-71365900"]
}

variable "tags_name" {
    default = "vault"
    description = "Name of tag"
}

variable "tags_billing" {
    default = "dev"
    description = "Billing name"
}

variable "tags_environment" {
    default = "dev"
    description = "Environment name"
}

variable "tags_product" {
    default = "Spectrum"
    description = "Product name"
}

variable "tags_timezone" {
    default = "Pacific Standard Time"
    description = "Instance timezone"
}

variable "vpc-id" {
    description = "VPC ID"
}

variable "deploy-pem-path" {
    description = "path of the deploy.pem file"
}
