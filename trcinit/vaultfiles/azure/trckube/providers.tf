terraform {
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 3.37.0"
    }
  }
}
provider "azurerm" {
  features {}
}

provider "tls" {}
