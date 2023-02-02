output "resource_group_name" {
  value = azurerm_resource_group.rg.name
}

output "public_ip_address" {
  value = azurerm_linux_virtual_machine.az-vm.public_ip_address
}

output "run_project" {
  value = "Login: ssh -i private_key.pem ubuntu@${azurerm_linux_virtual_machine.az-vm.public_ip_address}"
}

output "tls_private_key" {
  value     = tls_private_key.private_key.private_key_pem
  sensitive = true
}
