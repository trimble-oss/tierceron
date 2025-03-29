#!/usr/bin

# dev
trcplgtool -addr=$VAULT_ADDR -token=$VAULT_TOKEN -env=dev -defineService -pluginName=spiralis -projectservice="Hive/PluginSpiralis" -pluginType=trcshservice -codeBundle=spiralis.so -deployroot=/usr/local/trcshk -deploysubpath=plugins
