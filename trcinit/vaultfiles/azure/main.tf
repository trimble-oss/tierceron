resource "azurerm_resource_group" "rg" {
  name     = var.resource_group_name
  location = var.resource_group_location

  tags = {
    Environment = "Spectrum-Vault"
    Team        = "DevOps"
  }
}

#network protocols
# Create virtual network
resource "azurerm_virtual_network" "myterraformnetwork" {
  name                = "myVnet"
  address_space       = ["10.0.0.0/16"]
  location            = azurerm_resource_group.rg.location
  resource_group_name = azurerm_resource_group.rg.name
}

# Create subnet
resource "azurerm_subnet" "myterraformsubnet" {
  name                 = "mySubnet"
  resource_group_name  = azurerm_resource_group.rg.name
  virtual_network_name = azurerm_virtual_network.myterraformnetwork.name
  address_prefixes     = ["10.0.1.0/24"]
}

# Create public IPs
resource "azurerm_public_ip" "myterraformpublicip" {
  name                = "myPublicIP"
  location            = azurerm_resource_group.rg.location
  resource_group_name = azurerm_resource_group.rg.name
  allocation_method   = "Dynamic"
}

# Create Network Security Group and rule
resource "azurerm_network_security_group" "myterraformnsg" {
  name                = "myNetworkSecurityGroup"
  location            = azurerm_resource_group.rg.location
  resource_group_name = azurerm_resource_group.rg.name

  security_rule {
    name                       = "SSH"
    priority                   = 1001
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "22"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }
}

# Create network interface
resource "azurerm_network_interface" "myterraformnic" {
  name                = "myNIC"
  location            = azurerm_resource_group.rg.location
  resource_group_name = azurerm_resource_group.rg.name

  ip_configuration {
    name                          = "myNicConfiguration"
    subnet_id                     = azurerm_subnet.myterraformsubnet.id
    private_ip_address_allocation = "Dynamic"
    public_ip_address_id          = azurerm_public_ip.myterraformpublicip.id
  }
}

# Connect the security group to the network interface
resource "azurerm_network_interface_security_group_association" "example" {
  network_interface_id      = azurerm_network_interface.myterraformnic.id
  network_security_group_id = azurerm_network_security_group.myterraformnsg.id
}

# Generate random text for a unique storage account name
resource "random_id" "randomId" {
  keepers = {
    # Generate a new ID only when a new resource group is defined
    resource_group = azurerm_resource_group.rg.name
  }

  byte_length = 8
}


# Create storage account for boot diagnostics
resource "azurerm_storage_account" "mystorageaccount" {
  name                     = "diag${random_id.randomId.hex}"
  location                 = azurerm_resource_group.rg.location
  resource_group_name      = azurerm_resource_group.rg.name
  account_tier             = "Standard"
  account_replication_type = "LRS"
}


# Create (and display) an SSH key
resource "tls_private_key" "example_ssh" {
  algorithm = "RSA"
  rsa_bits  = 4096
}

# Create virtual machine
resource "azurerm_linux_virtual_machine" "myterraformvm" {
  name                  = "myVM"
  location              = azurerm_resource_group.rg.location
  resource_group_name   = azurerm_resource_group.rg.name
  network_interface_ids = [azurerm_network_interface.myterraformnic.id]
  size                  = "Standard_B1ls"

  os_disk {
    name                 = "myOsDisk"
    caching              = "ReadWrite"
    storage_account_type = "Premium_LRS"
  }

  source_image_reference {
    publisher = "Canonical"
    offer     = "UbuntuServer"
    sku       = "18.04-LTS"
    version   = "latest"
  }

  computer_name                   = "myvm"
  admin_username                  = "ubuntu"
  disable_password_authentication = true

  admin_ssh_key {
    username   = "ubuntu"
    public_key = tls_private_key.example_ssh.public_key_openssh
  }

  boot_diagnostics {
    storage_account_uri = azurerm_storage_account.mystorageaccount.primary_blob_endpoint
  }


  # Connections and provisioners MUST be inside of the vm block
  # In order to have multiple connections, the connection must
  # nested inside of the provisioner.
  provisioner "file" {
    connection {
      #host = "${azurerm_resource_group.rg.public_ip_address}"
      host        = self.public_ip_address
      user        = "ubuntu"
      type        = "ssh"
      private_key = file("id_rsa")
      timeout     = "10s"
      #agent = false
    }
    source      = "../resources/vault_properties.hcl"
    destination = "/tmp/vault_properties.hcl"
  }

  provisioner "file" {
    connection {
      #host = "${azurerm_resource_group.rg.public_ip_address}"
      host        = self.public_ip_address
      user        = "ubuntu"
      type        = "ssh"
      private_key = file("id_rsa")
      timeout     = "10s"
      #agent = false
    }
    #was previously serv_cert.pem
    source      = "../resources/cert.pem"
    destination = "/tmp/serv_cert.pem"
  }

  provisioner "file" {
    connection {
      #host = "${azurerm_resource_group.rg.public_ip_address}"
      host        = self.public_ip_address
      user        = "ubuntu"
      type        = "ssh"
      private_key = file("id_rsa")
      timeout     = "10s"
    }
    #was previously serv_key.pem
    source      = "../resources/key.pem"
    destination = "/tmp/serv_key.pem"
  }

 

 
  provisioner "file" {
    connection {
      #host = "${azurerm_resource_group.rg.public_ip_address}"
      host        = self.public_ip_address
      user        = "ubuntu"
      type        = "ssh"
      private_key = file("id_rsa")
      timeout     = "10s"
    }
    #source      = "${path.module}/scripts/install.sh"
    destination = "/tmp/install.sh"
    content     = templatefile(
      #inject variables into 
      "${path.module}/scripts/install.sh.tpl",
      {
        "TODOPORT" : var.TODOPORT
        "TODO" : var.TODO
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
      #"sudo export TODO=${var.TODO}",
      #"sudo export TODOPORT=${var.TODOPORT}"
    ]
    connection {
      host        = self.public_ip_address
      user        = "ubuntu"
      type        = "ssh"
      private_key = file("id_rsa")
      agent       = false
      timeout     = "10s"
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
      private_key = file("id_rsa")
      agent       = false
      timeout     = "10s"
    }
  }

}



