#!/bin/bash

#cd ../Github/tierceron/trcvault
cd ../
make configdbplugin
cd ../../Vault.Hashicorp

vault secrets disable vaultdb/

vault plugin deregister trc-vault-plugin
rm vault/data/core/plugin-catalog/secret/_trc-vault-plugin

cp ../Github/tierceron/bin/trc-vault-plugin plugins/trc-vault-plugin

cd plugins

vault plugin register \
          -command=trc-vault-plugin \
          -sha256=$( sha256sum trc-vault-plugin |cut -d' ' -f1 ) \
          -args=`backendUUID=4` \
          trc-vault-plugin

vault secrets enable \
          -path=vaultdb \
          -plugin-name=trc-vault-plugin \
          -description="Tierceron Vault Plugin" \
          plugin
