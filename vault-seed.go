package main

import (
	"flag"
	"fmt"
)

func main() {
	dirPtr := flag.String("dir", "seeds", "Directory containing seed files for vault")

	flag.Parse()
	fmt.Println(*dirPtr)
}
