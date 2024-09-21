#bin/bash 

trcplgtool -addr=$VAULT_ADDR -token=$VAULT_TOKEN -env=dev -defineService -pluginName=trcshm -projectservice="TrcVault/Trcshm" -pluginType=vault -codeBundle=trcshm -deployroot=$VAULT_PLUGIN_DEPLOY_ROOT -newRelicAppName="$NEWRELIC_APP_NAME" -newRelicLicenseKey=$NEWRELIC_LICENSE_KEY

trcplgtool -addr=$VAULT_ADDR -token=$VAULT_TOKEN -env=dev -defineService -pluginName=trcshq -pluginType=agent -codeBundle=trcshm -deployroot=/home/azuredeploy/bin

trcplgtool -addr=$VAULT_ADDR -token=$VAULT_TOKEN -env=dev -defineService -pluginName=trcshk -projectservice="Hive/Kernel" -pluginType=trcshpluginservice -codeBundle=trcshk -deployroot=/usr/local/trcshk