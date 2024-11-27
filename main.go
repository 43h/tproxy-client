package main

import (
	"flag"
	"fmt"
)

var logDebug = flag.Bool("d", false, "debug mode")
var version = flag.Bool("v", false, "print version and exit")

func main() {
	flag.Parse()
	if *version {
		fmt.Println("v0.0.1-20241127")
		return
	}
	if initLog(*logDebug) == false {
		return
	}
	defer closeLog()

	if initConf() == false {
		return
	}

	if initClient() == false {
		return
	}
	go startClient()
	defer closeClient()

	if initProxy() == false {
		return
	}
	startProxy()
	defer closeProxy()
}
