#!/bin/sh
set -eu

for environment in dev QA; do
    trcplgtool -addr="$VAULT_ADDR" -token="$VAULT_TOKEN" -env="$environment" -defineService -pluginName=talkctl -trcbootstrap=/deploy/deploy.trc -projectservice="Hive/PluginTalkctl" -pluginType=trcshcmdtoolplugin -codeBundle=talkctl.so -deployroot=/usr/local/trcshk -deploysubpath=plugins
done