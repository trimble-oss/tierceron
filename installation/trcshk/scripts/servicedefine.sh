#bin/bash 

trcplgtool -addr=$VAULT_ADDR -token=$VAULT_TOKEN -env=dev -defineService -pluginName=trcshk -projectservice="Hive/Kernel" -pluginType=trcshpluginservice -codeBundle=trcshk -deployroot=/usr/local/trcshk