package expirationDelete

import (
	"distributed-in-memory-db/gossip"
	"distributed-in-memory-db/skiplist"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

type Node struct {
	Address string `yaml:"address"`
}

type ExpirationDelete struct {
	ExpirationTime time.Duration
	nodes          []gossip.Node
	currentNode    Node
	config         Config
}

type Config struct {
	ExpirationTime time.Duration `yaml:"expiration_time"`
}

func NewExpirationDelete(config Config, GossipConfig gossip.Config, currentNode Node) *ExpirationDelete {
	return &ExpirationDelete{
		ExpirationTime: config.ExpirationTime,
		nodes:          GossipConfig.Nodes,
		currentNode:    currentNode,
		config:         config,
	}
}

func (edel *ExpirationDelete) ExpirationDeleteStart(db *skiplist.SkipList, id int) error {

	time.Sleep(edel.ExpirationTime)
	db.Delete(id)
	for _, node := range edel.nodes {
		fmt.Println(node.Address + edel.currentNode.Address)
		if node.Address == edel.currentNode.Address {
			continue
		}
		url := "http://" + node.Address + "/ExpirationDelete?id=" + strconv.Itoa(id)
		resp, err := http.Get(url)
		fmt.Println(url)
		fmt.Println(resp)
		if err != nil {
			return err
		}
		defer func(Body io.ReadCloser) {
			err := Body.Close()
			if err != nil {
				fmt.Println("Failed to close body" + err.Error())
			}
		}(resp.Body)
	}
	return nil
}
