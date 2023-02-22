resource "azurerm_resource_group" "rg" {
  name     = var.resource_group_name
  location = var.resource_group_location
}

data "azurerm_resource_group" "trg" {
  name     = var.resource_group_name_trg
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

resource "azurerm_virtual_network" "kubeVN" {
  name                = "${var.resource_group_name}-kubernetes-VN"
  resource_group_name = var.resource_group_name
  address_space       = [var.VN_rg_addr]
  location            = var.resource_group_location
  depends_on = [azurerm_resource_group.rg]
}

data "azurerm_virtual_network" "agentVN" {
  name                = "${var.VN_trg_name}"
  resource_group_name = var.resource_group_name_trg
  depends_on = [azurerm_resource_group.rg]
}

resource "azurerm_virtual_network_peering" "peerKubetoAgent" {
  name                      = "peerKubetoAgent"
  resource_group_name       = var.resource_group_name
  virtual_network_name      = azurerm_virtual_network.kubeVN.name
  remote_virtual_network_id = data.azurerm_virtual_network.agentVN.id
    depends_on = [
      azurerm_virtual_network.kubeVN,
      data.azurerm_virtual_network.agentVN
    ]
}

resource "azurerm_virtual_network_peering" "peerAgenttoKube" {
  name                      = "peerAgenttoKube"
  resource_group_name       = var.resource_group_name_trg
  virtual_network_name      = data.azurerm_virtual_network.agentVN.name
  remote_virtual_network_id = azurerm_virtual_network.kubeVN.id
  depends_on = [
      azurerm_virtual_network.kubeVN,
      data.azurerm_virtual_network.agentVN
    ]
}
