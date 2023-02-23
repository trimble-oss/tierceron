locals {
  locationCode          = var.resource_group_location == "East US 2" ? "EUS2" : "WUS2"
  envshort              = "dev-qa"
  productenv            = "Tierceron-${upper(local.envshort)}"
  max_count             = 2
  min_count             = 1
  rgname                = "${local.productenv}-${local.locationCode}-AKS-RG"
}

resource "azurerm_resource_group" "rg" {
  name     = var.resource_group_name
  location = var.resource_group_location
}

data "azurerm_subnet" "clusterSubnet" {
  name                 = var.VN_subnet_name
  virtual_network_name = var.VN_name
  resource_group_name  = var.resource_group_name_trg
}

data "azurerm_virtual_network" "virtual-network" {
  name                = var.VN_name
  resource_group_name = var.resource_group_name_trg
}

resource "azurerm_kubernetes_cluster" "tierceron_aks_cluster" {
  name                = "${local.productenv}-${local.locationCode}-AKS"
  location            = var.resource_group_location
  resource_group_name = local.rgname
  dns_prefix          = "${var.dnsprefix}"
  kubernetes_version        = var.Kube_version
  private_cluster_enabled   = true
  automatic_channel_upgrade = "patch"
  
  timeouts {
    delete = "1m" #Shared resource usually, so time out quickly if it is...
  }

  default_node_pool {
    name       = "kubepool${lower(local.locationCode)}"  
    enable_auto_scaling = true
    node_count = var.node_count
    vm_size    = var.vm_size
    type                = "VirtualMachineScaleSets"
    max_count           = local.max_count
    min_count           = local.min_count
    os_sku              = "Ubuntu"
    zones               = ["3"]
    vnet_subnet_id      = data.azurerm_subnet.clusterSubnet.id
  }

  network_profile {
    network_plugin      = "azure"
    service_cidrs       = [var.service_cidrs]
#    dns_service_ip      = var.dns_service_ip TODO: why???
    docker_bridge_cidr  = var.docker_bridge_cidr
    load_balancer_sku   = "standard"
  }

  identity {
    type = "SystemAssigned"
  }

  tags = {
    Application = "${var.resource_group_name}-kubernetes"
    Environment         = var.environment
  }

  depends_on = [azurerm_resource_group.rg]
}

data "azurerm_resource_group" "trg" {
  name     = var.resource_group_name_trg
}

