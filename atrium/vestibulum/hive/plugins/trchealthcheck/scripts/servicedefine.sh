#!/usr/bin

# dev
trcplgtool -addr=$VAULT_ADDR -token=$VAULT_TOKEN -env=dev -defineService -pluginName=healthcheck -trcbootstrap=/deploy/deploy.trc -projectservice="Hive/PluginHealthcheck" -pluginType=trcshpluginservice -codeBundle=healthcheck.so -deployroot=/usr/local/trcshk -deploysubpath=plugins
