data "azurerm_resource_group" "rg" {
  name     = var.resource_group_name
}

data "azurerm_virtual_network" "virtual-network" {
  name                = var.vm_db_VN
  resource_group_name = var.VN_rg_name
}

data "azurerm_subnet" "vm-subnet" {
  name                 = var.VM_subnet_name
  resource_group_name  = var.VN_rg_name
  virtual_network_name = var.vm_db_VN
}

data "azurerm_subnet" "db-subnet" {
 name                 = var.DB_subnet_name
  resource_group_name  = var.VN_rg_name
  virtual_network_name = var.vm_db_VN
}

// If you are not using custom DNS, you will need to link every zone you 
// want to use, to every VNET in your environment where you want the private
// endpoint resolution to work.

// azurerm_virtual_network_dns_servers

resource "azurerm_private_dns_zone_virtual_network_link" "vm-virtual-network-link" {
  name                  = "${var.subresource_group_name}-vm-virtual-network-link"
  resource_group_name   = data.azurerm_resource_group.rg.name
  private_dns_zone_name = azurerm_private_dns_zone.tierceron-dns-zone.name
  virtual_network_id    = data.azurerm_virtual_network.virtual-network.id
  registration_enabled  = false

  tags                  = {
     "Environment" = "${var.environment_short}"
     "Product"     = "${var.product}"
  }
}

resource "azurerm_private_dns_zone_virtual_network_link" "vm-db-virtual-network-link" {
  name                  = "${var.subresource_group_name}-vm-virtual-network-link"
  resource_group_name   = data.azurerm_resource_group.rg.name
  private_dns_zone_name = azurerm_private_dns_zone.tierceron-db-dns-zone.name
  virtual_network_id    = data.azurerm_virtual_network.virtual-network.id
tags                  = {
     "Environment" = "${var.environment_short}"
     "Product"     = "${var.product}"
  }  
}

resource "azurerm_public_ip" "public-ip" {
  name                = "${var.subresource_group_name}-PublicIP"
  location            = data.azurerm_resource_group.rg.location
  resource_group_name = data.azurerm_resource_group.rg.name
  allocation_method   = "Static"

  tags = {
    "Application" = var.subresource_group_name
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
  name                = "${var.subresource_group_name}-NetworkSecurityGroup"
  location            = data.azurerm_resource_group.rg.location
  resource_group_name = data.azurerm_resource_group.rg.name

  tags = {
    "Application" = var.subresource_group_name
    "Billing"     = var.environment
    "Environment" = "${var.environment_short}"
    "Product"     = "${var.product}"
  }

  #SSH INBOUND - Restrict to allowed IPs and Port(s)
  security_rule {
    name                       = "Allow${var.org_name}SshInbound"
    description                = ""
    priority                   = 110
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "22"
    source_address_prefix      = ""
    source_address_prefixes    = var.allowed_ip_ranges
    destination_address_prefix = "*"
  }

  #TCP INBOUND VAULT - Restrict to allowed IPs and Port(s)
  security_rule {
    name                       = "Allow${var.org_name}IpsInbound"
    description                = ""
    priority                   = 120
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = ""
    destination_port_ranges    = var.dest_port_ranges
    source_address_prefix     = ""
    source_address_prefixes    = var.allowed_ip_ranges
    destination_address_prefix = "*"
  }

  #UDP INBOUND DNS
  security_rule {
    access                     = "Allow"
    description                = ""
    destination_address_prefix = "*"
    destination_address_prefixes               = []
    destination_application_security_group_ids = []
    destination_port_range     = ""
    destination_port_ranges    = ["53"]
    direction                  = "Inbound"
    name                       = "Allow${var.org_name}UdpInbound"
    priority                   = 130
    protocol                   = "Udp"
    source_address_prefix      = "*"
    source_address_prefixes    = []
    source_application_security_group_ids      = []
    source_port_range          = "*"
    source_port_ranges                         = []
  }

  #SSH OUTBOUND - Restrict to allowed IPs on Port 22
  security_rule {
    access                                     = "Allow"
    description                                = ""
    destination_address_prefix                 = var.allowed_vpn_ip_ranges[0]
    destination_address_prefixes               = []
    destination_application_security_group_ids = []
    destination_port_range                     = "22"
    destination_port_ranges                    = []
    direction                                  = "Outbound"
    name                                       = "Allow${var.org_name}-VPN-SshOutbound"
    priority                                   = 110
    protocol                                   = "Tcp"
    source_address_prefix                      = "*"
    source_address_prefixes                    = []
    source_application_security_group_ids      = []
    source_port_range                          = "*"
    source_port_ranges                         = []
  }

  security_rule {
     access                                     = "Allow"
     description                                = ""
     destination_address_prefix                 = var.allowed_ip_ranges[0]
     destination_address_prefixes               = []
     destination_application_security_group_ids = []
     destination_port_range                     = "22"
     destination_port_ranges                    = []
     direction                                  = "Outbound"
     name                                       = "Allow${var.org_name}-Corp-SshOutbound"
     priority                                   = 120
     protocol                                   = "Tcp"
     source_address_prefix                      = "*"
     source_address_prefixes                    = []
     source_application_security_group_ids      = []
     source_port_range                          = "*"
     source_port_ranges                         = []
  }

  #TCP OUTBOUND VAULT - Restrict to allowed IPs on all ports - Narrow this down if needed.
  security_rule {
     access                                     = "Allow"
     description                                = ""
     destination_address_prefix                 = var.allowed_vpn_ip_ranges[0]
     destination_address_prefixes               = []
     destination_application_security_group_ids = []
     destination_port_range                     = "*"
     destination_port_ranges                    = []
     direction                                  = "Outbound"
     name                                       = "Allow${var.org_name}-VPN-TcpOutbound"
     priority                                   = 130
     protocol                                   = "Tcp"
     source_address_prefix                      = "*"
     source_address_prefixes                    = []
     source_application_security_group_ids      = []
     source_port_range                          = "*"
     source_port_ranges                         = []
  }

  security_rule {
     access                                     = "Allow"
     description                                = ""
     destination_address_prefix                 = var.allowed_ip_ranges[0]
     destination_address_prefixes               = []
     destination_application_security_group_ids = []
     destination_port_range                     = "*"
     destination_port_ranges                    = []
     direction                                  = "Outbound"
     name                                       = "Allow${var.org_name}-Corp-TcpOutbound"
     priority                                   = 140
     protocol                                   = "Tcp"
     source_address_prefix                      = "*"
     source_address_prefixes                    = []
     source_application_security_group_ids      = []
     source_port_range                          = "*"
     source_port_ranges                         = []
  }

  #UDP OUTBOUND DNS
  security_rule {
    access                        = "Allow"
    description                   = ""
    destination_address_prefix    = "*"
    destination_address_prefixes  = []
    destination_application_security_group_ids = []
    destination_port_range        = ""
    destination_port_ranges       = ["22", "53"]
    direction                     = "Outbound"
    name                          = "Allow${var.org_name}UdpOutbound"
    priority                      = 150
    protocol                      = "Udp"
    source_address_prefix         = "*"
    source_address_prefixes       = []
    source_application_security_group_ids      = []
    source_port_range             = "*"
    source_port_ranges            = []
  }
}



resource "azurerm_network_interface" "vm-network-interface" {
  name                = "${var.subresource_group_name}-NIC"
  location            = data.azurerm_resource_group.rg.location
  resource_group_name = data.azurerm_resource_group.rg.name

  ip_configuration {
    name                          = "${var.subresource_group_name}-NicConfiguration"
    subnet_id                     = data.azurerm_subnet.vm-subnet.id
    private_ip_address_allocation = "Dynamic"
    public_ip_address_id          = azurerm_public_ip.public-ip.id
  }

  tags = {
    "Environment" = "${var.environment_short}"
    "Product"     = "${var.product}"
    "Application" = var.subresource_group_name
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
resource "azurerm_network_interface_security_group_association" "tierceron-security-group" {
  network_interface_id      = azurerm_network_interface.vm-network-interface.id
  network_security_group_id = azurerm_network_security_group.nsg.id
}

# TODO: this creates the wrong kind of record...
# I want name.domain1 --> name.domain2
# This creates name.domain1->name.name.domain1 -- or some such.

# resource "azurerm_private_dns_cname_record" "tierceron-cname" {
#  name                = "${var.tierceronname}.${var.tierceronzone}"
#  zone_name           = azurerm_private_dns_zone.tierceron-dns-zone.name
#  resource_group_name = data.azurerm_resource_group.rg.name
#  ttl                 = 300
#  record              = "${var.tierceronname}"
#  depends_on = [
#    azurerm_private_dns_zone.tierceron-dns-zone
#  ]
#}

resource "azurerm_private_dns_zone" "tierceron-db-dns-zone" {
  name                = "${var.dbzone}"
  resource_group_name = data.azurerm_resource_group.rg.name
  tags = {
     "Environment" = "${var.environment_short}"
     "Product"     = "${var.product}"
     "Application" = var.subresource_group_name
     "Billing"     = var.environment
  }
}

resource "azurerm_private_dns_zone" "tierceron-dns-zone" {
  name                = "${var.tierceronzone}"
  resource_group_name = data.azurerm_resource_group.rg.name
  tags = {
    "Environment" = "${var.environment_short}"
    "Product"     = "${var.product}"
    "Application" = var.subresource_group_name
    "Billing"     = var.environment
  }
}

resource "azurerm_mysql_flexible_server" "tierceron-db" {
  name                   = "${var.dbaddress}"
  resource_group_name    = data.azurerm_resource_group.rg.name
  location               = data.azurerm_resource_group.rg.location
  administrator_login    = "${var.mysql_admin}"
  administrator_password = "${var.mysql_admin_password}"
  backup_retention_days  = "${var.mysql_backup_retention_days}"
  delegated_subnet_id    = data.azurerm_subnet.db-subnet.id
  private_dns_zone_id    = azurerm_private_dns_zone.tierceron-db-dns-zone.id
  sku_name               = "B_Standard_B2s"
  zone                   = "3"
  count                  = "${var.make_flexible_server}" == "yes" ? 1 : 0

  tags = {
    "Environment" = "${var.environment_short}"
    "Product"     = "${var.product}"
  }

  storage {
    auto_grow_enabled = true
  }
  depends_on = [
   azurerm_private_dns_zone.tierceron-dns-zone
  ]
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
  name                  = "${var.subresource_group_name}-vm"
  location              = data.azurerm_resource_group.rg.location
  resource_group_name   = data.azurerm_resource_group.rg.name
  network_interface_ids = [azurerm_network_interface.vm-network-interface.id]
  size                  = "${var.vault_vm_size}"

  os_disk {
    name                 = "${var.subresource_group_name}-OsDisk"
    caching              = "ReadWrite"
    storage_account_type = "Premium_LRS"
  }

  source_image_reference {
    publisher = "Canonical"
    offer     = "0001-com-ubuntu-server-focal"
    sku       = "20_04-lts-gen2"
    version   = "20.04.202302070"
  }

  computer_name         = "${var.tierceronname}"
  admin_username                  = "ubuntu"
  disable_password_authentication = true

  tags = {
    "CreatedBy"   = "${var.created_by}"
    "CreatedOn"   = "${var.created_on}"
    "Environment" = "${var.environment_short}"
    "Product"     = "${var.product}"
    "backup"      = "external"
  }

  identity {
    identity_ids = []
    type         = "SystemAssigned"
  }

  admin_ssh_key {
    username   = "ubuntu"
    public_key = tls_private_key.private_key.public_key_openssh
  }

  provisioner "local-exec" {
    interpreter = ["bash", "-c"]
    command = <<EOT
      echo ${var.trcdb_provisioner}
      rm resources/vault_properties.sub
      sed 's/TRCDBNAME/${var.trcdb_provisioner}/g' resources/vault_properties.hcl > resources/vault_properties.sub
    EOT
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
    source      = "resources/vault_properties.sub"
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
        "HOSTPORT"        = var.hostport
        "VAULTIP"         = var.vaultip
        "CONTROLLERA_PORT" = var.controllera_port
        "CONTROLLERB_PORT" = var.controllerb_port
        "TRCDBA_PORT"     = var.trcdba_port
        "TRCDBB_PORT"     = var.trcdbb_port
        "HOST"            = var.host
        "write_service"   = var.write_service
        "SSH_PORT"        = var.ssh_port
        "SCRIPT_CIDR_BLOCK" = var.script_cidr_block
        "ONSITE_CIDR_BLOCK" = var.onsite_cidr_block
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
    "Environment" = "${var.environment_short}"
    "Product"     = "${var.product}"
    "Application" = var.subresource_group_name
    "Billing"     = var.environment
  }

  settings = <<SETTINGS
    {
        "commandToExecute": "${var.security_software_script_path} | sudo bash"
    }
SETTINGS
}


