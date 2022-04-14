#!/bin/bash

echo "Enter plugin name: "
read TRC_PLUGIN_NAME

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

echo "Precertify plugin: "
read PRE_CERTIFY

if [ "$VAULT_ENV" = "prod" ] || [ "$VAULT_ENV" = "staging" ]; then
trcplgtool -env=$VAULT_ENV -checkDeployed -addr=$VAULT_ADDR -token=$VAULT_TOKEN -insecure -pluginName=$TRC_PLUGIN_NAME-prod -sha256=$(cat target/$TRC_PLUGIN_NAME-prod.sha256)
else 
trcplgtool -env=$VAULT_ENV -checkDeployed -addr=$VAULT_ADDR -token=$VAULT_TOKEN -insecure -pluginName=$TRC_PLUGIN_NAME -sha256=$(cat target/$TRC_PLUGIN_NAME.sha256)
fi 

if [ "$?" = 0 ]; then       
echo "This version of the plugin has already been deployed - enabling for environment $VAULT_ENV."
vault write vaultdb/$VAULT_ENV token=$VAULT_ENV_TOKEN
exit $?
fi

if [ "$?" = 1 ]; then       
echo "Unable to validate if existing plugin has been deployed - cannot continue."
exit $?
fi

if [ "$?" = 2 ]; then       
echo "This version of the plugin has not been deployed for environment $VAULT_ENV."
fi

vault secrets disable vaultdb/
vault plugin deregister $TRC_PLUGIN_NAME


if [ "$VAULT_ENV" = "prod" ] || [ "$VAULT_ENV" = "staging" ]; then

if [ "$PRE_CERTIFY" = "Y" ] || [ "$PRE_CERTIFY" = "yes" ]; then
trcplgtool -env=$VAULT_ENV -certify -addr=$VAULT_ADDR -token=$VAULT_TOKEN -insecure -pluginName=$TRC_PLUGIN_NAME-prod -sha256=$(cat target/$TRC_PLUGIN_NAME-prod.sha256)
fi

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

if [ "$PRE_CERTIFY" = "Y" ] || [ "$PRE_CERTIFY" = "yes" ]; then
trcplgtool -env=$VAULT_ENV -certify -addr=$VAULT_ADDR -token=$VAULT_TOKEN -insecure -pluginName=$TRC_PLUGIN_NAME -sha256=$(cat target/$TRC_PLUGIN_NAME.sha256)
fi

# Just writing to vault will trigger a copy...
# First we set Copied to false...
# This should also trigger the copy process...
# It should return sha256 of copied plugin on success.
#SHA256BUNDLE=$(vault write vaultcarrier/$VAULT_ENV token=$VAULT_ENV_TOKEN plugin=$TRC_PLUGIN_NAME)
#SHAVAL=$(echo $SHA256BUNDLE | awk '{print $6}')
SHAVAL=$( cat target/$TRC_PLUGIN_NAME.sha256 )

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

