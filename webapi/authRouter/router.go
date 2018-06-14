package main

import (
	"net/http"

	//"github.com/dgrijalva/jwt-go"
	twp "bitbucket.org/dexterchaney/whoville/webapi/rpc/twirpapi"
	rtr "github.com/julienschmidt/httprouter"
)

func route(routeHandler http.Handler) {

}

func main() {
	twirpClient := twp.
	
	.NewTwirpAPIProtobufClient("http://localhost:8080", &http.Client{})
	route(twirpClien)
}
