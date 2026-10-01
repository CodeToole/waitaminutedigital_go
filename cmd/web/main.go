package main

import (
	"log"

	"github.com/CodeToole/waitaminutedigital_go/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Load configuration: %v", err)
	}

	addr := ":" + cfg.Port
	log.Printf("Starting Waitaminute Digital server on %s", addr)
	if err := newServer().Start(addr); err != nil {
		log.Fatalf("Server stopped: %v", err)
	}
}
