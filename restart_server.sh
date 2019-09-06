#!/bin/sh
sudo pkill -f apiRouter
#kill -9 `pgrep apiRouter`
sudo ./apiRouter -token=$1 &