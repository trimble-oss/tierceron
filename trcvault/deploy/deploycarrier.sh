#!/bin/bash

if [[ -z "${VAULT_ADDR}" ]]; then
echo "Enter vault host base url: "
read VAULT_ADDR
fi

if [[ -z "${VAULT_TOKEN}" ]]; then
echo "Enter root token: "
read VAULT_TOKEN
fi

if [[ -z "${VAULT_ENV}" ]]; then
echo "Enter environment: "
read VAULT_ENV
fi

if [[ -z "${VAULT_ENV_TOKEN}" ]]; then
echo "Enter environment token with write permissions: "
read VAULT_ENV_TOKEN
fi

vault secrets disable vaultcarrier/
vault plugin deregister trc-vault-carrier-plugin

# TODO: remove this next line...  or parameterize it.
cp target/trc-vault-carrier-plugin ../../../../Vault.Hashicorp/plugins/

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
