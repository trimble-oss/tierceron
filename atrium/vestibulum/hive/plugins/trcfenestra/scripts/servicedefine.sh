#!/usr/bin

# dev
trcplgtool -addr=$VAULT_ADDR -token=$VAULT_TOKEN -env=dev -defineService -pluginName=fenestra -projectservice="Hive/PluginFenestra" -pluginType=trcshservice -codeBundle=fenestra.so -deployroot=/usr/local/trcshk -deploysubpath=plugins
