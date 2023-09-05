package main

import (
    "log"
    "golang_proxy/proxy"
)

func main() {
	const address = ":3333"
	log.Printf("Starting proxy at: %s\n", address)
    proxy := proxy.Proxy{}
    err := proxy.Start(address)
    if err != nil {
        log.Printf("%s\n", err)
    }
}
