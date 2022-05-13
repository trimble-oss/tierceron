#!/bin/bash

echo "Enter vault host base url: "
read VAULT_ADDR

echo "Enter root token: "
read VAULT_TOKEN

export VAULT_ADDR
export VAULT_TOKEN

vault secrets disable vaultcarrier/

vault plugin deregister trc-vault-carrier-plugin
#rm vault/data/core/plugin-catalog/secret/_trc-vault-carrier-plugin
#rm plugins/*

