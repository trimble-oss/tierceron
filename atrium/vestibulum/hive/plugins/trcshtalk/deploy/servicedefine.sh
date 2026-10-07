#bin/bash 

trcplgtool -addr=$VAULT_ADDR -token=$VAULT_TOKEN -env=dev -defineService -pluginName=trcshtalk -projectservice="Hive/PluginTrcshTalk" -pluginType=trcshpluginservice -codeBundle=trcshtalk.so -deployroot=/usr/local/trcshk/plugins

trcplgtool -addr=$VAULT_ADDR -token=$VAULT_TOKEN -env=QA -defineService -pluginName=trcshtalk -projectservice="Hive/PluginTrcshTalk" -pluginType=trcshpluginservice -codeBundle=trcshtalk.so -deployroot=/usr/local/trcshk/plugins

trcplgtool -addr=$VAULT_ADDR -token=$VAULT_TOKEN -env=performance -defineService -pluginName=trcshtalk -projectservice="Hive/PluginTrcshTalk" -pluginType=trcshpluginservice -codeBundle=trcshtalk.so -deployroot=/usr/local/trcshk/plugins