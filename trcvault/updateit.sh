#!/bin/bash

#cd ../Github/tierceron/trcvault
cd ../
make configdbplugin
cd ../../Vault.Hashicorp

VERSION=`date -u +.%Y%m%d.%H%M%S`

cp ../Github/tierceron/bin/trc-vault-plugin plugins/trc-vault-plugin-$VERSION
cd plugins

vault plugin register \
          -command=trc-vault-plugin-$VERSION \
          -sha256=$( sha256sum trc-vault-plugin-$VERSION |cut -d' ' -f1 ) \
          -args=`backendUUID=4` \
          trc-vault-plugin

vault secrets enable \
          -path=vaultdb \
          -plugin-name=trc-vault-plugin \
          -description="Tierceron Vault Plugin" \
          plugin

curl \
    --header "X-Vault-Token: $VAULT_TOKEN" \
    --request PUT \
    --data '{ "plugin": "trc-vault-plugin" }' \
    --insecure \
    https://vault.whoboot.org:8200/v1/sys/plugins/reload/backend
