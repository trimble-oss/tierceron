#!/bin/bash
#get bamboo artifacts: apiRouter and vaultUI
#set up credentials??
#sudo rm /etc/opt/vaultAPI/apiRouter
#sudo rm -r /etc/opt/vaultAPI/public
#sudo aws configure --profile default
BUILD=$1
sudo rm /etc/opt/vaultAPI/apiRouter
sudo rm -r /etc/opt/vaultAPI/public
sudo aws s3 cp s3://dexterchaney-atlassian-net-bamboo-artifacts/bamboo-artifacts/plan-39354400/shared/build-$BUILD/VaultUI/VaultUI.zip /etc/opt/vaultAPI/public.zip --profile default
sudo aws s3 cp s3://dexterchaney-atlassian-net-bamboo-artifacts/bamboo-artifacts/plan-39354400/shared/build-$BUILD/apiRouter/apiRouter /etc/opt/vaultAPI/apiRouter --profile default
#sudo mv /etc/opt/vaultAPI/apiRouter /etc/opt/vaultAPI/apiRouterEx
sudo chmod +x /etc/opt/vaultAPI/apiRouter
sudo mkdir -p /etc/opt/vaultAPI/public
sudo unzip /etc/opt/vaultAPI/public.zip -d /etc/opt/vaultAPI/public
sudo rm /etc/opt/vaultAPI/public.zip
#put artifacts in the proper location