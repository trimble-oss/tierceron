#!/usr/bin

# dev
trcplgtool -addr=$VAULT_ADDR -token=$VAULT_TOKEN -env=dev -defineService -pluginName=trchelloworld -projectservice="HelloProject/HelloService" -pluginType=trcshservice -serviceName="TomcatMain" -codeBundle=trchelloword -deployroot=/usr/local/hello -deploysubpath=bin
