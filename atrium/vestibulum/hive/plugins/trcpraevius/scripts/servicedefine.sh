#!/usr/bin

# dev
trcplgtool -addr=$VAULT_ADDR -token=$VAULT_TOKEN -env=dev -defineService -pluginName=praevius -trcbootstrap=/deploy/deploy.trc -projectservice="Hive/Pluginpraevius" -pluginType=trcshpluginservice -codeBundle=praevius.so -deployroot=/usr/local/trcshk -deploysubpath=plugins
