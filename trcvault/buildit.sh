#!/bin/bash

#cd ../Github/tierceron/trcvault
cd ../
make configdbplugin
cd bin
sha256sum trc-vault-plugin | cut -d' ' -f1 > trc-vault-plugin.sha256

cd ../../../Vault.Hashicorp

vault secrets disable vaultdb/

vault plugin deregister trc-vault-plugin
rm vault/data/core/plugin-catalog/secret/_trc-vault-plugin

cp ../Github/tierceron/bin/trc-vault-plugin plugins/trc-vault-plugin

echo "Registering plugin"
vault plugin register \
          -command=trc-vault-plugin \
          -sha256=$( cat ../Github/tierceron/bin/trc-vault-plugin.sha256 ) \
          -args=`backendUUID=4` \
          trc-vault-plugin

echo "Plugin registered"

vault secrets enable \
          -path=vaultdb \
          -plugin-name=trc-vault-plugin \
          -description="Tierceron Vault Plugin" \
          plugin
