#!/usr/bin

# dev
trcplgtool -addr=$VAULT_ADDR -token=$VAULT_TOKEN -env=dev -defineService -pluginName=mutabilis -trcbootstrap=/deploy/deploy.trc -projectservice="Hive/PluginMutabilis" -pluginType=trcshpluginservice -codeBundle=mutabilis.so -deployroot=/usr/local/trcshk -deploysubpath=plugins
