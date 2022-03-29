#!/bin/bash

echo "Enter vault host base url: "
read VAULT_ADDR

#echo "Enter vault private ip: "
#read VAULT_PRIVATE_IP

echo "Enter root token: "
read VAULT_TOKEN

echo "Enter environment: "
read VAULT_ENV

echo "Enter environment token with write permissions: "
read VAULT_ENV_TOKEN

# scp -i ~/pems/deploy.pem ubuntu@"$VAULT_PRIVATE_IP":/etc/opt/vault/plugins

if [ "$VAULT_ENV" = "prod" ] || [ "$VAULT_ENV" = "staging" ] # || [ "$VAULT_ENV" = "QA" ]
then
vault plugin register \
          -command=trc-vault-plugin-prod \
          -sha256=$( cat target/trc-vault-plugin-prod.sha256 ) \
          -args=`backendUUID=4` \
          trc-vault-plugin-prod
vault secrets enable \
          -path=vaultdb \
          -plugin-name=trc-vault-plugin-prod \
          -description="Tierceron Vault Plugin Prod" \
          plugin
else
vault plugin register \
          -command=trc-vault-plugin \
          -sha256=$( cat target/trc-vault-plugin.sha256 ) \
          -args=`backendUUID=4` \
          trc-vault-plugin
vault secrets enable \
          -path=vaultdb \
          -plugin-name=trc-vault-plugin \
          -description="Tierceron Vault Plugin" \
          plugin
fi

vault write vaultdb/$VAULT_ENV token=$VAULT_ENV_TOKEN
