#!/usr/bin

# dev
trcplgtool -addr=$VAULT_ADDR -token=$VAULT_TOKEN -env=dev -defineService -pluginName=rosea -trcbootstrap=/deploy/deploy.trc -projectservice="Hive/PluginRosea" -pluginType=trcshcmdtoolplugin -codeBundle=rosea.so -deployroot=/usr/local/trcshk -deploysubpath=plugins
