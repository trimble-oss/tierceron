#!/bin/bash

cd ../../../Vault.Hashicorp

echo "Enter vault host base url: "
read VAULT_ADDR

echo "Enter root token: "
read VAULT_TOKEN

echo "Enter plugin name: "
read TRC_PLUGIN_NAME

export VAULT_ADDR
export VAULT_TOKEN
export TRC_PLUGIN_NAME

vault secrets disable $TRC_PLUGIN_NAME/

vault plugin deregister $TRC_PLUGIN_NAME
#rm vault/data/core/plugin-catalog/secret/_trc-vault-plugin
#rm plugins/*

