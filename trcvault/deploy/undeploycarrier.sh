#!/bin/bash


if [[ -z "${VAULT_ADDR}" ]]; then
echo "Enter vault host base url: "
read VAULT_ADDR
fi

if [[ -z "${VAULT_TOKEN}" ]]; then
echo "Enter root token: "
read VAULT_TOKEN
fi

export VAULT_ADDR
export VAULT_TOKEN

vault secrets disable vaultcarrier/

vault plugin deregister trc-vault-carrier-plugin
#rm vault/data/core/plugin-catalog/secret/_trc-vault-carrier-plugin
#rm plugins/*

