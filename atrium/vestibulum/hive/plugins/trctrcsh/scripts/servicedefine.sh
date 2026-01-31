#!/usr/bin

# dev
trcplgtool -addr=$VAULT_ADDR -token=$VAULT_TOKEN -env=dev -defineService -pluginName=trcsh -trcbootstrap=/deploy/deploy.trc -projectservice="Hive/PluginTrcsh" -pluginType=trcshcmdtoolplugin -codeBundle=trcsh.so -deployroot=/usr/local/trcshk -deploysubpath=plugins

#QA
trcplgtool -addr=$VAULT_ADDR -token=$VAULT_TOKEN -env=QA -defineService -pluginName=trcsh -trcbootstrap=/deploy/deploy.trc -projectservice="Hive/PluginTrcsh" -pluginType=trcshcmdtoolplugin -codeBundle=trcsh.so -deployroot=/usr/local/trcshk -deploysubpath=plugins
