#!/bin/sh
go install "github.com/trimble-oss/tierceron/webapi/apiRouter"
zip ./../../webapi/apiRouter/apiRouter.zip ~/bin/apiRouter