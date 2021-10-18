package main

import (
	"flag"

	"grpc_test/grpc_demo/client"
	"grpc_test/grpc_demo/service"
)

func main() {
	var (
		t = flag.Int("type", 0, "0: server; 1: client")
	)
	flag.Parse()
	switch *t {
	case 0:
		service.Start()
	case 1:
		client.Start()
	}
}
