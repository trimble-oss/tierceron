#!/usr/bin

# dev
trcplgtool -addr=$VAULT_ADDR -token=$VAULT_TOKEN -env=dev -defineService -pluginName=procurator -trcbootstrap=/deploy/deploy.trc -projectservice="Hive/PluginProcurator" -pluginType=trcshpluginservice -codeBundle=procurator.so -deployroot=/usr/local/trcshk -deploysubpath=plugins
