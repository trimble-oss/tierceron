#!/bin/bash

echo "Enter plugin name: "
read TRC_PLUGIN_NAME

echo "Enter vault host base url: "
read VAULT_ADDR

echo "Enter root token: "
read VAULT_TOKEN

echo "Enter environment: "
read VAULT_ENV

echo "Enter environment token with write permissions: "
read VAULT_ENV_TOKEN

echo "Precertify plugin: "
read PRE_CERTIFY

if [ "$VAULT_ENV" = "prod" ] || [ "$VAULT_ENV" = "staging" ]; then

if [ "$PRE_CERTIFY" = "Y" ] || [ "$PRE_CERTIFY" = "yes" ]; then
trcplgtool -env=$VAULT_ENV -certify -addr=$VAULT_ADDR -token=$VAULT_TOKEN -insecure -pluginName=$TRC_PLUGIN_NAME-prod -sha256=$(cat target/$TRC_PLUGIN_NAME-prod.sha256)
fi

else

if [ "$PRE_CERTIFY" = "Y" ] || [ "$PRE_CERTIFY" = "yes" ]; then
trcplgtool -env=$VAULT_ENV -certify -addr=$VAULT_ADDR -token=$VAULT_TOKEN -insecure -pluginName=$TRC_PLUGIN_NAME -sha256=$(cat target/$TRC_PLUGIN_NAME.sha256)
fi

fi

echo "Checking plugin deploy status."
if [ "$VAULT_ENV" = "prod" ] || [ "$VAULT_ENV" = "staging" ]; then
echo "Certifying plugin for env $VAULT_ENV."
trcplgtool -env=$VAULT_ENV -checkDeployed -addr=$VAULT_ADDR -token=$VAULT_TOKEN -insecure -pluginName=$TRC_PLUGIN_NAME-prod -sha256=$(cat target/$TRC_PLUGIN_NAME-prod.sha256)
status=$?
echo "Plugin certified with result $status."
else
echo "Certifying plugin for env $VAULT_ENV."
trcplgtool -env=$VAULT_ENV -checkDeployed -addr=$VAULT_ADDR -token=$VAULT_TOKEN -insecure -pluginName=$TRC_PLUGIN_NAME -sha256=$(cat target/$TRC_PLUGIN_NAME.sha256)
status=$?
echo "Plugin certified with result $status."
fi 

if [ $status -eq 0 ]; then       
echo "This version of the plugin has already been deployed - enabling for environment $VAULT_ENV."
vault write vaultdb/$VAULT_ENV token=$VAULT_ENV_TOKEN
exit $status
elif [ $status -eq 1 ]; then
echo "Existing plugin does not match repository plugin - cannot continue."
exit $status
elif [ $status -eq 2 ]; then
echo "This version of the plugin has not been deployed for environment $VAULT_ENV.  Beginning installation process."
else
echo "Unexpected error response."
exit $status
fi
echo "Uninstalling existing plugin."

VAULT_API_ADDR=VAULT_ADDR
export VAULT_ADDR
export VAULT_API_ADDR

echo "Disable old trc vault secrets"
vault secrets disable vaultdb/
vault plugin deregister $TRC_PLUGIN_NAME


if [ "$VAULT_ENV" = "prod" ] || [ "$VAULT_ENV" = "staging" ]; then

# Just writing to vault will trigger the carrier plugin...
# First we set Copied to false...
# This should also trigger the copy process...
# It should return sha256 of copied plugin on success.
SHA256BUNDLE=$(vault write vaultcarrier/$VAULT_ENV token=$VAULT_ENV_TOKEN plugin=$TRC_PLUGIN_NAME-prod)
SHAVAL=$(echo $SHA256BUNDLE | awk '{print $6}')

echo "Registering new plugin."
vault plugin register \
          -command=$TRC_PLUGIN_NAME-prod \
          -sha256=$(echo $SHAVAL) \
          -args=`backendUUID=789` \
          $TRC_PLUGIN_NAME-prod
echo "Enabling new plugin."
vault secrets enable \
          -path=vaultdb \
          -plugin-name=$TRC_PLUGIN_NAME-prod \
          -description="Tierceron Vault Plugin Prod" \
          plugin
else

# Just writing to vault will trigger a copy...
# First we set Copied to false...
# This should also trigger the copy process...
# It should return sha256 of copied plugin on success.
SHA256BUNDLE=$(vault write vaultcarrier/$VAULT_ENV token=$VAULT_ENV_TOKEN plugin=$TRC_PLUGIN_NAME)
SHAVAL=$(echo $SHA256BUNDLE | awk '{print $6}')
#SHAVAL=$( cat target/$TRC_PLUGIN_NAME.sha256 )
if [ "$VAULT_PLUGIN_DIR" ]
then
sudo setcap cap_ipc_lock=+ep $VAULT_PLUGIN_DIR/$TRC_PLUGIN_NAME
fi

echo "Registering new plugin."
vault plugin register \
          -command=$TRC_PLUGIN_NAME \
          -sha256=$(echo $SHAVAL) \
          -args=`backendUUID=789` \
          $TRC_PLUGIN_NAME
echo "Enabling new plugin."
vault secrets enable \
          -path=vaultdb \
          -plugin-name=$TRC_PLUGIN_NAME \
          -description="Tierceron Vault Plugin" \
          plugin
fi

#Activates/starts the deployed plugin.
# Note: plugin should update deployed flag for itself.
vault write vaultdb/$VAULT_ENV token=$VAULT_ENV_TOKEN

