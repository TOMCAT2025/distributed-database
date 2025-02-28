package main

import (
	"distributed-in-memory-db/node"
	"flag"
	"log"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to the configuration file")
	flag.Parse()

	//创建数据库节点
	server, err := node.NewNode(*configPath)
	if err != nil {
		log.Fatalf("Failed to create node: %v", err)
	}
	server.Start()
}
