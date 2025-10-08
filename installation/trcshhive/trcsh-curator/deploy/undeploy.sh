#!/bin/bash

echo "Enter plugin name: "
read TRC_PLUGIN_NAME

echo "Enter vault host base url: "
read VAULT_ADDR

echo "Enter root token: "
read VAULT_TOKEN

export VAULT_ADDR
export VAULT_TOKEN
export TRC_PLUGIN_NAME

vault secrets disable $TRC_PLUGIN_NAME/
vault secrets list | grep $TRC_PLUGIN_NAME
existingplugin=$?
if [ $existingplugin -eq 0 ]; then       
    echo "Plugin still mounted unexpectedly.  Manual intervention required to clean up before deregistration can proceed."
    exit 1
else
    echo "All mounts cleared.  Continuing..."
fi

vault plugin deregister secret $TRC_PLUGIN_NAME

