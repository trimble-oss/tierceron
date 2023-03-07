#!/bin/bash

echo "Enter plugin name: "
read TRC_PLUGIN_NAME

FILE="target/$TRC_PLUGIN_NAME"
if [ ! -f "$FILE" ]; then
    echo "$FILE does not exist."
    exit 1
fi

FILESHA="target/$TRC_PLUGIN_NAME.sha256"
if [ ! -f "$FILESHA" ]; then
    echo "$FILESHA does not exist."
    exit 1
fi

FILESHAVAL=$(cat $FILESHA)

echo "Enter vault host base url: "
read VAULT_ADDR

echo "Enter root token: "
read VAULT_TOKEN

echo "Enter environment: "
read VAULT_ENV

echo "Enter trc plugin runtime environment token with write permissions unrestricted: "
read VAULT_ENV_TOKEN

echo "Is this plugin an agent (Y or N): "
read VAULT_AGENT

if [ "$VAULT_PLUGIN_DIR" ]
then
    echo "Deploying using local vault strategy."
    PRE_CERTIFY="N"
else
    echo "Precertify plugin (Y or N): "
    read PRE_CERTIFY
fi

if [ "$VAULT_ENV" = "prod" ] || [ "$VAULT_ENV" = "staging" ]; then
    if [ "$PRE_CERTIFY" = "Y" ] || [ "$PRE_CERTIFY" = "yes" ] || [ "$PRE_CERTIFY" = "y" ]; then
        if [ "$VAULT_AGENT" = 'Y' ] || [ "$VAULT_AGENT" = 'y' ]; then 
            echo "Deploying agent"
            trcplgtool -env=$VAULT_ENV -certify -addr=$VAULT_ADDR -token=$VAULT_TOKEN -insecure -pluginName=$TRC_PLUGIN_NAME-prod -sha256=$(cat target/$TRC_PLUGIN_NAME-prod.sha256) -pluginType=agent
            echo "Agent has been copied and certified"
        else
            trcplgtool -env=$VAULT_ENV -certify -addr=$VAULT_ADDR -token=$VAULT_TOKEN -insecure -pluginName=$TRC_PLUGIN_NAME-prod -sha256=$(cat target/$TRC_PLUGIN_NAME-prod.sha256)
        fi
    fi
else
    if [ "$PRE_CERTIFY" = "Y" ] || [ "$PRE_CERTIFY" = "yes" ] || [ "$PRE_CERTIFY" = "y" ]; then
        if [ "$VAULT_AGENT" = 'Y' ] || [ "$VAULT_AGENT" = 'y' ]; then 
	        echo "Deploying agent"
            trcplgtool -env=$VAULT_ENV -certify -addr=$VAULT_ADDR -token=$VAULT_TOKEN -insecure -pluginName=$TRC_PLUGIN_NAME -sha256=$(cat target/$TRC_PLUGIN_NAME.sha256) -pluginType=agent
            echo "Agent has been copied and certified"
        else
            trcplgtool -env=$VAULT_ENV -certify -addr=$VAULT_ADDR -token=$VAULT_TOKEN -insecure -pluginName=$TRC_PLUGIN_NAME -sha256=$(cat target/$TRC_PLUGIN_NAME.sha256)
        fi
    fi
fi

if [ "$VAULT_AGENT" = 'Y' ] || [ "$VAULT_AGENT" = 'y' ]; then
    exit 0
fi

if [ "$VAULT_PLUGIN_DIR" ]
then
    echo "Local plugin registration skipping certification."
else
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
    vault write $TRC_PLUGIN_NAME/$VAULT_ENV token=$VAULT_ENV_TOKEN vaddress=$VAULT_ADDR
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
fi
echo "Uninstalling existing plugin."

VAULT_API_ADDR=VAULT_ADDR
export VAULT_ADDR
export VAULT_TOKEN
export VAULT_API_ADDR

echo "Disable and unregister old plugin."
export VAULT_CLIENT_TIMEOUT=300s
# Migrate to carrier maybe?
vault secrets disable $TRC_PLUGIN_NAME/
vault plugin deregister $TRC_PLUGIN_NAME

sleep 12

if [ "$VAULT_ENV" = "prod" ] || [ "$VAULT_ENV" = "staging" ]; then
    # Just writing to vault will trigger the carrier plugin...
    # First we set Copied to false...
    # This should also trigger the copy process...
    # It should return sha256 of copied plugin on success.
    SHA256BUNDLE=$(vault write vaultcarrier/$VAULT_ENV plugin=$TRC_PLUGIN_NAME-prod)
    SHAVAL=$(echo $SHA256BUNDLE | awk '{print $6}')
    
    if [ "$SHAVAL" != "$FILESHAVAL" ]; then
    echo "Carrier failed to deploy plugin.  Please try again."
    exit -1
    fi

    if [ "$SHAVAL" = "" ]; then
    echo "Failed to obtain sha256 for indicated plugin.   Refusing to continue."
    exit -1
    fi

    echo "Registering new plugin."
    # This can be replaced with? https://developer.hashicorp.com/vault/api-docs/system/plugins-catalog#register-plugin
    # POST to: curl -H "X-Vault-Token: $VAULT_TOKEN" https://vault.dexchadev.com:8020/v1/sys/plugins/catalog/secret/trc-vault-plugin
    # Migrate to carrier maybe?
    vault plugin register \
            -command=$TRC_PLUGIN_NAME-prod \
            -sha256=$(echo $SHAVAL) \
            -args=`backendUUID=789` \
            $TRC_PLUGIN_NAME-prod
    echo "Enabling new plugin."
    # This can be replaces with? https://developer.hashicorp.com/vault/api-docs/v1.4.x/system/mounts#enable-secrets-engine
    # POST to: curl -H "X-Vault-Token: $VAULT_TOKEN" https://127.0.0.1:10001/v1/sys/mounts
    # Migrate to carrie maybe?
    vault secrets enable \
            -path=$TRC_PLUGIN_NAME \
            -plugin-name=$TRC_PLUGIN_NAME-prod \
            -description="Tierceron Vault Plugin Prod" \
            plugin
else
    if [ "$VAULT_PLUGIN_DIR" ]
    then
        chmod 700 target/$TRC_PLUGIN_NAME
        cp target/$TRC_PLUGIN_NAME $VAULT_PLUGIN_DIR
        SHAVAL=$( cat target/$TRC_PLUGIN_NAME.sha256 )
        sudo setcap cap_ipc_lock=+ep $VAULT_PLUGIN_DIR/$TRC_PLUGIN_NAME
    else
        # Just writing to vault will trigger a copy...
        # First we set Copied to false...
        # This should also trigger the copy process...
        # It should return sha256 of copied plugin on success.
        SHA256BUNDLE=$(vault write vaultcarrier/$VAULT_ENV plugin=$TRC_PLUGIN_NAME)
        SHAVAL=$(echo $SHA256BUNDLE | awk '{print $6}')

        if [ "$SHAVAL" = "Failure" ]; then
            echo "Failed to obtain sha256 for indicated plugin.   Refusing to continue."
            exit -1
        fi
    fi

    if [ "$SHAVAL" != "$FILESHAVAL" ]; then
    echo "Carrier failed to deploy plugin.  Please try again."
    exit -1
    fi

    echo "Registering new plugin."
    vault plugin register \
            -command=$TRC_PLUGIN_NAME \
            -sha256=$(echo $SHAVAL) \
            -args=`backendUUID=789` \
            $TRC_PLUGIN_NAME
    echo "Enabling new plugin."
    vault secrets enable \
            -path=$TRC_PLUGIN_NAME \
            -plugin-name=$TRC_PLUGIN_NAME \
            -description="Tierceron Vault Plugin" \
            plugin
fi


#Activates/starts the deployed plugin.
# Note: plugin should update deployed flag for itself.
vault write $TRC_PLUGIN_NAME/$VAULT_ENV token=$VAULT_ENV_TOKEN vaddress=$VAULT_ADDR

