#!/usr/bin

# dev
trcplgtool -addr=$VAULT_ADDR -token=$VAULT_TOKEN -env=dev -defineService -pluginName=descartes -trcbootstrap=/deploy/deploy.trc -projectservice="Hive/PluginDescartes" -pluginType=trcshpluginservice -codeBundle=descartes.so -deployroot=/usr/local/trcshk -deploysubpath=plugins
