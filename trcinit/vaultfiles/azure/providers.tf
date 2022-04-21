# Configure the Azure provider
terraform {
  required_providers {
    #azure resource manager
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 3.0.2"
    }
  }

  required_version = ">= 1.1.0"
}

# If you wish to use the default behaviours of the Azure Provider, 
# then you only need to define an empty features block
provider "azurerm" {
  features {}
}