#!/bin/bash

echo "Enter vault host base url: "
read VAULT_ADDR

echo "Enter root token: "
read VAULT_TOKEN

echo "Enter environment: "
read VAULT_ENV

echo "Enter environment token with write permissions: "
read VAULT_ENV_TOKEN

VAULT_API_ADDR=VAULT_ADDR
export VAULT_ADDR
export VAULT_API_ADDR

echo "Disable old carrier secrets"
vault secrets disable vaultcarrier/
echo "Unregister old carrier plugin"
vault plugin deregister trc-vault-carrier-plugin

if [ "$VAULT_ADDR" = "https://vault.whoboot.org:8200" ]
then
cp target/trc-vault-carrier-plugin ../../../../Vault.Hashicorp/plugins/
fi

if [ "$VAULT_ENV" = "prod" ] || [ "$VAULT_ENV" = "staging" ]
then
vault plugin register \
          -command=trc-vault-carrier-plugin-prod \
          -sha256=$( cat target/trc-vault-carrier-plugin-prod.sha256 ) \
          -args=`backendUUID=567` \
          plugin
vault secrets enable \
          -path=vaultcarrier \
          -plugin-name=trc-vault-carrier-plugin-prod \
          -description="Tierceron Vault Carrier Plugin Prod" \
          plugin
else
echo "Registering Carrier"
vault plugin register \
          -command=trc-vault-carrier-plugin \
          -sha256=$( cat target/trc-vault-carrier-plugin.sha256 ) \
          -args=`backendUUID=567` \
          trc-vault-carrier-plugin
echo "Enabling Carrier secret engine"
vault secrets enable \
          -path=vaultcarrier \
          -plugin-name=trc-vault-carrier-plugin \
          -description="Tierceron Vault Carrier Plugin" \
          plugin
fi
