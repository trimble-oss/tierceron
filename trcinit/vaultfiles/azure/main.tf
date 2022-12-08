resource "azurerm_resource_group" "rg" {
  name     = var.resource_group_name
  location = var.resource_group_location

  tags = {
    "Application" = var.resource_group_name
    "Billing"     = var.environment
  }
}



resource "azurerm_virtual_network" "vm-virtual-network" {
  name                = "${var.resource_group_name}-vm-Vnet"
  address_space       = ["10.0.0.0/16"]
  location            = azurerm_resource_group.rg.location
  resource_group_name = azurerm_resource_group.rg.name

  tags = {
    "Application" = var.resource_group_name
    "Billing"     = var.environment
  }
}

resource "azurerm_virtual_network" "db-virtual-network" {
  name                = "${var.resource_group_name}-db-Vnet"
  address_space       = ["10.1.0.0/16"]
  location            = azurerm_resource_group.rg.location
  resource_group_name = azurerm_resource_group.rg.name

  tags = {
    "Application" = var.resource_group_name
    "Billing"     = var.environment
  }
}

resource "azurerm_private_dns_zone_virtual_network_link" "db-virtual-network-link" {
  name                  = "${var.resource_group_name}-db-virtual-network-link"
  resource_group_name   = azurerm_resource_group.rg.name
  private_dns_zone_name = azurerm_private_dns_zone.tierceron-dns-zone.name
  virtual_network_id    = azurerm_virtual_network.db-virtual-network.id
}


resource "azurerm_virtual_network_peering" "peer-db-vm" {
  name                      = "${var.resource_group_name}-peerVMToDb"
  resource_group_name       = azurerm_resource_group.rg.name
  virtual_network_name      = azurerm_virtual_network.db-virtual-network.name
  remote_virtual_network_id = azurerm_virtual_network.vm-virtual-network.id
#  lifecycle  {
#    replace_triggered_by = [azurerm_virtual_network.peer-db-vm.address_space, azurerm_virtual_network.peer-vm-db.address_space]
#  }
}

resource "azurerm_virtual_network_peering" "peer-vm-db" {
  name                      = "${var.resource_group_name}-peerDbToVm"
  resource_group_name       = azurerm_resource_group.rg.name
  virtual_network_name      = azurerm_virtual_network.vm-virtual-network.name
  remote_virtual_network_id = azurerm_virtual_network.db-virtual-network.id
 # lifecycle  {
 #   replace_triggered_by = [azurerm_virtual_network.peer-vm-db.address_space, azurerm_virtual_network.peer-db-vm.address_space]
 # }
}

resource "azurerm_subnet" "vm-subnet" {
  name                 = "${var.resource_group_name}-subnet"
  resource_group_name  = azurerm_resource_group.rg.name
  virtual_network_name = azurerm_virtual_network.vm-virtual-network.name
  address_prefixes     = ["10.0.0.0/24"]
}

resource "azurerm_subnet" "db-subnet" {
  name                 = "${var.resource_group_name}-db-subnet"
  resource_group_name  = azurerm_resource_group.rg.name
  virtual_network_name = azurerm_virtual_network.db-virtual-network.name
  address_prefixes     = ["10.1.0.0/24"]
  service_endpoints    = ["Microsoft.Storage"]

  delegation {
    name = "fs"
    service_delegation {
        name = "Microsoft.DBforMySQL/flexibleServers"
        actions = [
          "Microsoft.Network/virtualNetworks/subnets/join/action",
        ]
      }
  }
}



resource "azurerm_public_ip" "public-ip" {
  name                = "${var.resource_group_name}-PublicIP"
  location            = azurerm_resource_group.rg.location
  resource_group_name = azurerm_resource_group.rg.name
  allocation_method   = "Static"

  tags = {
    "Application" = var.resource_group_name
    "Billing"     = var.environment
  }

  # Prevent terraform from changing static ip address on apply.
  # New vpn firewall rules won't be necessesary on rebuild.
  # Comment out to allow ip changes when running terraform apply.
  lifecycle {
    ignore_changes = [
      name,
      location,
      resource_group_name,
      allocation_method,
      tags,
    ]
  }
}



# Create Network Security Group and rule
resource "azurerm_network_security_group" "nsg" {
  name                = "${var.resource_group_name}-NetworkSecurityGroup"
  location            = azurerm_resource_group.rg.location
  resource_group_name = azurerm_resource_group.rg.name

  tags = {
    "Application" = var.resource_group_name
    "Billing"     = var.environment
  }

  #SSH INBOUND - Restrict to allowed IPs and Port(s)
  security_rule {
    name                       = "Allow${var.org_name}SshInbound"
    priority                   = 110
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "22"
    source_address_prefix      = var.allowed_ips
    destination_address_prefix = "*"
  }

  #TCP INBOUND VAULT - Restrict to allowed IPs and Port(s)
  security_rule {
    name                       = "Allow${var.org_name}IpsInbound"
    priority                   = 111
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = var.dest_port_range
    source_address_prefix      = var.allowed_ips
    destination_address_prefix = "*"
  }

  #SSH OUTBOUND - Restrict to allowed IPs on Port 22
  security_rule {
    name                       = "Allow${var.org_name}SshOutbound"
    priority                   = 110
    direction                  = "Outbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "22"
    source_address_prefix      = "*"
    destination_address_prefix = var.allowed_ips
  }

  #TCP OUTBOUND VAULT - Restrict to allowed IPs on all ports - Narrow this down if needed.
  security_rule {
    name                       = "Allow${var.org_name}TcpOutbound"
    priority                   = 111
    direction                  = "Outbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "*"
    source_address_prefix      = "*"
    destination_address_prefix = var.allowed_ips
  }
}



resource "azurerm_network_interface" "vm-network-interface" {
  name                = "${var.resource_group_name}-NIC"
  location            = azurerm_resource_group.rg.location
  resource_group_name = azurerm_resource_group.rg.name

  ip_configuration {
    name                          = "${var.resource_group_name}-NicConfiguration"
    subnet_id                     = azurerm_subnet.vm-subnet.id
    private_ip_address_allocation = "Dynamic"
    public_ip_address_id          = azurerm_public_ip.public-ip.id
  }

  tags = {
    "Application" = var.resource_group_name
    "Billing"     = var.environment
  }

  # Prevent terraform from changing static ip address on apply.
  # New vpn firewall rules won't be necessesary on rebuild.
  # Comment out to allow ip changes.
  lifecycle {
    ignore_changes = [
      ip_configuration["public_ip_address"],
    ]
  }
}



# Connect the security group to the network interface
resource "azurerm_network_interface_security_group_association" "example" {
  network_interface_id      = azurerm_network_interface.vm-network-interface.id
  network_security_group_id = azurerm_network_security_group.nsg.id
}

resource "azurerm_private_dns_zone" "tierceron-dns-zone" {
  name                = "${var.dbaddress}"
  resource_group_name = azurerm_resource_group.rg.name
  tags = {
    "Application" = var.resource_group_name
    "Billing"     = var.environment
  }
}

resource "azurerm_mysql_flexible_server" "tiercercon-db" {
  name                   = "tiercercon-db"
  resource_group_name    = azurerm_resource_group.rg.name
  location               = azurerm_resource_group.rg.location
  administrator_login    = "${var.mysql_admin}"
  administrator_password = "${var.mysql_admin_password}"
  backup_retention_days  = "${var.mysql_backup_retention_days}"
  delegated_subnet_id    = azurerm_subnet.db-subnet.id
  private_dns_zone_id    = azurerm_private_dns_zone.tierceron-dns-zone.id
  sku_name               = "B_Standard_B2s"

  storage {
    auto_grow_enabled = true
  }
  depends_on = [azurerm_private_dns_zone_virtual_network_link.db-virtual-network-link]
}

resource "tls_private_key" "private_key" {
  algorithm = "RSA"
  rsa_bits  = 4096
}



resource "local_file" "private_key" {
  content              = tls_private_key.private_key.private_key_pem
  filename             = "private_key.pem"
  file_permission      = "600"
  directory_permission = "755"

  # Remove ssh key when running terraform destroy.
  provisioner "local-exec" {
    when    = destroy
    command = "rm -f private_key.pem"
  }
}



resource "azurerm_linux_virtual_machine" "az-vm" {
  name                  = "${var.resource_group_name}-vm"
  location              = azurerm_resource_group.rg.location
  resource_group_name   = azurerm_resource_group.rg.name
  network_interface_ids = [azurerm_network_interface.vm-network-interface.id]
  size                  = "Standard_B1ls"

  os_disk {
    name                 = "${var.resource_group_name}-OsDisk"
    caching              = "ReadWrite"
    storage_account_type = "Premium_LRS"
  }

  source_image_reference {
    publisher = "Canonical"
    offer     = "UbuntuServer"
    sku       = "18.04-LTS"
    version   = "latest"
  }

  computer_name                   = "${var.resource_group_name}-vm"
  admin_username                  = "ubuntu"
  disable_password_authentication = true

  tags = {
    "Application" = var.resource_group_name
    "Billing"     = var.environment
  }


  admin_ssh_key {
    username   = "ubuntu"
    public_key = tls_private_key.private_key.public_key_openssh
  }

  # Connections and provisioners must be inside of the vm block
  # in order to have multiple connections. The connection for each
  # must be nested inside of the associated provisioner.
  provisioner "file" {
    connection {
      host        = self.public_ip_address
      user        = "ubuntu"
      type        = "ssh"
      private_key = tls_private_key.private_key.private_key_pem
      timeout     = "30s"
    }
    source      = "resources/vault_properties.hcl"
    destination = "/tmp/vault_properties.hcl"
  }

  provisioner "file" {
    connection {
      host        = self.public_ip_address
      user        = "ubuntu"
      type        = "ssh"
      private_key = tls_private_key.private_key.private_key_pem
      timeout     = "30s"
    }
    source      = "vault/cert.pem"
    destination = "/tmp/serv_cert.pem"
  }

  provisioner "file" {
    connection {
      host        = self.public_ip_address
      user        = "ubuntu"
      type        = "ssh"
      private_key = tls_private_key.private_key.private_key_pem
      timeout     = "30s"
    }
    source      = "vault/key.pem"
    destination = "/tmp/serv_key.pem"
  }

    provisioner "file" {
    connection {
      host        = self.public_ip_address
      user        = "ubuntu"
      type        = "ssh"
      private_key = tls_private_key.private_key.private_key_pem
      timeout     = "30s"
    }
    source      = "vault/sqlcert.pem"
    destination = "/tmp/DigiCertGlobalRootCA.crt.pem"
  }

  provisioner "file" {
    connection {
      host        = self.public_ip_address
      user        = "ubuntu"
      type        = "ssh"
      private_key = tls_private_key.private_key.private_key_pem
      timeout     = "30s"
    }
    

    destination = "/tmp/install.sh"
    content = templatefile(
      #inject variables into the install script via template file
      "${path.module}/scripts/install.sh.tpl",
      {
        "HOSTPORT"      = var.hostport
        "HOST"          = var.host
        "write_service" = var.write_service
        "SSH_PORT"      = var.ssh_port
        "SCRIPT_CIDR_BLOCK" = var.script_cidr_block
      }
    )
  }

  provisioner "remote-exec" {
    inline = [
      "sudo mkdir /tmp/public",
      "sudo chown ubuntu /tmp/public",
      "sudo mkdir /tmp/policy_files",
      "sudo chown ubuntu /tmp/policy_files",
      "sudo mkdir /tmp/token_files",
      "sudo chown ubuntu /tmp/token_files",
      "sudo mkdir /tmp/template_files",
      "sudo chown ubuntu /tmp/template_files",
    ]
    connection {
      host        = self.public_ip_address
      user        = "ubuntu"
      type        = "ssh"
      private_key = tls_private_key.private_key.private_key_pem
      agent       = false
      timeout     = "30s"
    }
  }

  provisioner "remote-exec" {
    inline = [
      "chmod +x /tmp/install.sh",
      "/tmp/install.sh"
    ]
    connection {
      host        = self.public_ip_address
      user        = "ubuntu"
      type        = "ssh"
      private_key = tls_private_key.private_key.private_key_pem
      agent       = false
      timeout     = "30s"
    }
  }
}



resource "azurerm_virtual_machine_extension" "security_software" {
  name                 = "${var.security_software_name}.install-${var.environment}"
  virtual_machine_id   = azurerm_linux_virtual_machine.az-vm.id
  publisher            = "Microsoft.Azure.Extensions"
  type                 = "CustomScript"
  type_handler_version = "2.0"

  tags = {
    "Application" = var.resource_group_name
    "Billing"     = var.environment
  }

  settings = <<SETTINGS
    {
        "commandToExecute": "${var.security_software_script_path} | sudo bash"
    }
SETTINGS
}


