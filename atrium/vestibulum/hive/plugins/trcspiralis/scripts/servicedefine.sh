#!/usr/bin

# dev
trcplgtool -addr=$VAULT_ADDR -token=$VAULT_TOKEN -env=dev -defineService -pluginName=spiralis -trcbootstrap=/deploy/deploy.trc -projectservice="Hive/PluginSpiralis" -pluginType=trcshcmdtoolplugin -codeBundle=spiralis.so -deployroot=/usr/local/trcshk -deploysubpath=plugins
