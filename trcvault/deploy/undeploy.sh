#!/bin/bash

cd ../../../Vault.Hashicorp

echo "Enter vault host base url: "
read VAULT_ADDR

echo "Enter root token: "
read VAULT_TOKEN

export VAULT_ADDR
export VAULT_TOKEN

vault secrets disable trc-vault-plugin/

vault plugin deregister trc-vault-plugin
#rm vault/data/core/plugin-catalog/secret/_trc-vault-plugin
#rm plugins/*

