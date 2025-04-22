#!/usr/bin

# dev
trcplgtool -addr=$VAULT_ADDR -token=$VAULT_TOKEN -env=dev -defineService -pluginName=spiralis -projectservice="Hive/PluginSpiralis" -pluginType=trcshcmdtoolplugin -codeBundle=spiralis.so -deployroot=/usr/local/trcshk -deploysubpath=plugins
