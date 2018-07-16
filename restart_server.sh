#!/bin/sh
sudo pkill -f apiRouter
sudo ~/bin/apiRouter -token=$1 -addr=http://localhost:8200 -auth &