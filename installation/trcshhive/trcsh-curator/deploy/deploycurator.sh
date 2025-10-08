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

PROD_EXT=""
for x in "staging" "prod"; do
    if [ $x = $VAULT_ENV ]; then
       PROD_EXT="-prod"
    fi
done

VAULT_API_ADDR=VAULT_ADDR
export VAULT_ADDR
export VAULT_TOKEN
export VAULT_API_ADDR

echo "Disable old curator secrets"
vault secrets disable vaultcurator/
vault secrets list | grep trcsh-curator$PROD_EXT
existingplugin=$?
if [ $existingplugin -eq 0 ]; then       
    echo "Curator plugin still mounted elsewhere.  Manual intervention required to clean up before registration can proceed."
    exit 1
else
    echo "All mounts cleared.  Continuing..."
fi
echo "Unregister old curator plugin"
vault plugin deregister trcsh-curator$PROD_EXT

if [ "$VAULT_PLUGIN_DIR" ]
then
echo "Copying new curator plugin"
cp target/trcsh-curator$PROD_EXT $VAULT_PLUGIN_DIR
chmod 700 $VAULT_PLUGIN_DIR/trcsh-curator$PROD_EXT
sudo setcap cap_ipc_lock=+ep $VAULT_PLUGIN_DIR/trcsh-curator$PROD_EXT
fi



echo "Registering Curator"
echo trcsh-curator$PROD_EXT

vault plugin register \
          -command=trcsh-curator$PROD_EXT \
          -sha256=$( cat target/trcsh-curator$PROD_EXT.sha256 ) \
          -args=`backendUUID=567` \
          trcsh-curator$PROD_EXT
echo "Enabling Curator secret engine"
vault secrets enable \
          -path=vaultcurator \
          -plugin-name=trcsh-curator$PROD_EXT \
          -description="Tierceron Vault Curator Plugin" \
          plugin
