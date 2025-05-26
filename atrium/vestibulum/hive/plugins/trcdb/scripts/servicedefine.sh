#!/usr/bin

# dev
trcplgtool -addr=$VAULT_ADDR -token=$VAULT_TOKEN -env=dev -defineService -pluginName=trcdb -trcbootstrap=/deploy/deploy.trc -projectservice="Hive/PluginTrcdb" -pluginType=trcflowpluginservice -codeBundle=trcdb.so -deployroot=/usr/local/trcshk -deploysubpath=plugins
