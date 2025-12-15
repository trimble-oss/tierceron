#!/usr/bin

# RQA
trcplgtool -addr=$VAULT_ADDR -token=$VAULT_TOKEN -env=RQA -defineService -pluginName=procurator -trcbootstrap=/deploy/deploy.trc -projectservice="Hive/PluginProcurator" -pluginType=trcshpluginservice -codeBundle=procurator.so -deployroot=/usr/local/trcshk/plugins
