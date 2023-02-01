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
    node_count = 2
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


resource "tls_private_key" "private_key" {
  algorithm = "RSA"
  rsa_bits  = 4096
}

resource "local_file" "private_key" {
  content              = tls_private_key.private_key.private_key_pem
  filename             = "kube_key.pem"
  file_permission      = "600"
  directory_permission = "755"
}

