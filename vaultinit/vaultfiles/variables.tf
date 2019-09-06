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

variable "vpc-id" {
    description = "VPC ID"
}

variable "deploy-pem-path" {
    description = "path of the deploy.pem file"
}
