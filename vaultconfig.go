package main

import (
	"flag"
	"fmt"
)

/*
This Vault configurator app will read and populate templates
*/
func main() {
	fmt.Println("Hello World")
	flag.String("accesstoken", "", "vault access token")
	flag.Parse()
}
