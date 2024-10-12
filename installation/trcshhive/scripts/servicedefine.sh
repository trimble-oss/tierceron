#bin/bash 

trcplgtool -addr=$VAULT_ADDR -token=$VAULT_TOKEN -env=dev -defineService -pluginName=trcsh-curator -projectservice="TrcVault/TrcshCurator" -pluginType=vault -codeBundle=trcsh-curator -deployroot=$VAULT_PLUGIN_DEPLOY_ROOT -newRelicAppName="$NEWRELIC_APP_NAME" -newRelicLicenseKey=$NEWRELIC_LICENSE_KEY

trcplgtool -addr=$VAULT_ADDR -token=$VAULT_TOKEN -env=dev -defineService -pluginName=trcsh-cursor-k -projectservice="TrcVault/TrcshCursorK" -pluginType=vault -codeBundle=trcsh-cursor-k -deployroot=$VAULT_PLUGIN_DEPLOY_ROOT -newRelicAppName="$NEWRELIC_APP_NAME" -newRelicLicenseKey=$NEWRELIC_LICENSE_KEY

trcplgtool -addr=$VAULT_ADDR -token=$VAULT_TOKEN -env=dev -defineService -pluginName=trcsh-cursor-aw -projectservice="TrcVault/TrcshCursorAW" -pluginType=vault -codeBundle=trcsh-cursor-aw -deployroot=$VAULT_PLUGIN_DEPLOY_ROOT -newRelicAppName="$NEWRELIC_APP_NAME" -newRelicLicenseKey=$NEWRELIC_LICENSE_KEY

trcplgtool -addr=$VAULT_ADDR -token=$VAULT_TOKEN -env=dev -defineService -pluginName=trcshqaw -pluginType=agent -codeBundle=trcshqaw -deployroot=/home/azuredeploy/bin

trcplgtool -addr=$VAULT_ADDR -token=$VAULT_TOKEN -env=dev -defineService -pluginName=trcshqk -pluginType=agent -codeBundle=trcshqk -deployroot=/home/azuredeploy/bin

trcplgtool -addr=$VAULT_ADDR -token=$VAULT_TOKEN -env=dev -defineService -pluginName=trcshk -projectservice="Hive/Kernel" -pluginType=trcshpluginservice -codeBundle=trcshk -deployroot=/usr/local/trcshk