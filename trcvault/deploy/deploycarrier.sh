#!/bin/bash

echo "Enter vault host base url: "
read VAULT_ADDR

echo "Enter root token: "
read VAULT_TOKEN

echo "Enter environment: "
read VAULT_ENV

echo "Enter environment token with write permissions: "
read VAULT_ENV_TOKEN

# TODO: make api call to trc carrier with sha256...
# input (plugin name)
# output sha256 or error (if plugin has not been copied and a copy attempt failed).

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
vault plugin register \
          -command=trc-vault-carrier-plugin \
          -sha256=$( cat target/trc-vault-carrier-plugin.sha256 ) \
          -args=`backendUUID=567` \
          trc-vault-carrier-plugin
vault secrets enable \
          -path=vaultcarrier \
          -plugin-name=trc-vault-carrier-plugin \
          -description="Tierceron Vault Carrier Plugin" \
          plugin
fi

vault write vaultcarrier/$VAULT_ENV token=$VAULT_ENV_TOKEN

# TODO: run trcplgtool -certify -deployed
