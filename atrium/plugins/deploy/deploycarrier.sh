#!/bin/bash

if [[ -z "${VAULT_ADDR}" ]]; then
echo "Enter agent vault host base url: "
read VAULT_ADDR
fi

if [[ -z "${VAULT_TOKEN}" ]]; then
echo "Enter agent root token: "
read VAULT_TOKEN
fi

if [[ -z "${VAULT_ENV}" ]]; then
echo "Enter organization/agent environment: "
read VAULT_ENV
fi

VAULT_API_ADDR=VAULT_ADDR
export VAULT_ADDR
export VAULT_TOKEN
export VAULT_API_ADDR

echo "Disable old carrier secrets"
vault secrets disable vaultcarrier/
vault secrets list | grep trc-vault-carrier-plugin
existingplugin=$?
if [ $existingplugin -eq 0 ]; then       
    echo "Carrier plugin still mounted elsewhere.  Manual intervention required to clean up before registration can proceed."
    exit 1
else
    echo "All mounts cleared.  Continuing..."
fi
echo "Unregister old carrier plugin"
vault plugin deregister trc-vault-carrier-plugin

if [ "$VAULT_PLUGIN_DIR" ]
then
echo "Copying new carrier plugin"
cp target/trc-vault-carrier-plugin $VAULT_PLUGIN_DIR
chmod 700 $VAULT_PLUGIN_DIR/trc-vault-carrier-plugin
sudo setcap cap_ipc_lock=+ep $VAULT_PLUGIN_DIR/trc-vault-carrier-plugin
fi

PROD_EXT=""
for x in "staging" "prod"; do
    if [ $x = $VAULT_ENV ]; then
       PROD_EXT="-prod"
    fi
done

echo "Registering Carrier"
echo trc-vault-carrier-plugin$PROD_EXT

vault plugin register \
          -command=trc-vault-carrier-plugin$PROD_EXT \
          -sha256=$( cat target/trc-vault-carrier-plugin.sha256 ) \
          -args=`backendUUID=567` \
          trc-vault-carrier-plugin
echo "Enabling Carrier secret engine"
vault secrets enable \
          -path=vaultcarrier \
          -plugin-name=trc-vault-carrier-plugin \
          -description="Tierceron Vault Carrier Plugin" \
          plugin
