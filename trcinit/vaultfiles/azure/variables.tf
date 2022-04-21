variable "environment" {
  default     = "develop"
  description = "Variable used to differentiate between multiple program environments"
}

variable "resource_group_location" {
  default     = "westus2"
  description = "Location of the resource group."
}

variable "resource_group_name" {
  default = "Spectrum"
}

#variable "deploy-pem-path" {
#    description = "path of the deploy.pem file"
#}