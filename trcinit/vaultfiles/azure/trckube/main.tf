resource "azurerm_resource_group" "rg" {
  name     = var.resource_group_name
  location = var.resource_group_location
}

resource "azurerm_kubernetes_cluster" "kcluster" {
  name                = "${var.resource_group_name}-kubernetes"
  location            = var.resource_group_location
  resource_group_name = var.resource_group_name
  dns_prefix          = "${var.dnsprefix}-${var.environment}-kube"
  private_cluster_enabled   = true
  tags = {
    "Application" = "${var.resource_group_name}-kubernetes"
    "Billing"     = var.environment
  }
   timeouts {
    delete = "1m" #Shared resource usually, so time out quickly if it is...
  }

  default_node_pool {
    name       = "default"  
    node_count = var.node_count
    vm_size    = var.vm_size  
  }

  identity {
    type = "SystemAssigned"
  }

  network_profile {
        load_balancer_sku = "standard"
        network_plugin = "kubenet"
  }

  depends_on = [azurerm_resource_group.rg]
}

resource "azurerm_resource_group" "trg" {
  name     = var.resource_group_name_trg
}

resource "azurerm_virtual_network" "virtual-network" {
  name                = var.VN_name
  resource_group_name = var.resource_group_name_trg
}

resource "azurerm_subnet" "vm-subnet" {
  name                 = var.VN_subnet_name
  resource_group_name  = var.resource_group_name_trg
  virtual_network_name = var.VN_name
}