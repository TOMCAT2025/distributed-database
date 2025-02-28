package node

import (
	"distributed-in-memory-db/api"
	"distributed-in-memory-db/expirationDelete"
	"distributed-in-memory-db/gossip"
	"distributed-in-memory-db/skiplist"
	"distributed-in-memory-db/storage"
	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
	"log"
	"os"
	"strconv"
)

type Config struct {
	ThreadModel          string                  `yaml:"thread_model"`
	ConsistencyAlgorithm string                  `yaml:"consistency_algorithm"`
	DataStructure        string                  `yaml:"data_structure"`
	Gossip               gossip.Config           `yaml:"gossip"`
	Database             storage.DBConfig        `yaml:"database"`
	Port                 int                     `yaml:"port"`
	ED                   expirationDelete.Config `yaml:"expiration_delete"`
}

type Node struct {
	port   int
	config Config
	db     *skiplist.SkipList
	gossip *gossip.Gossip
	router *gin.Engine
}

func NewNode(configPath string) (*Node, error) {
	//加载配置信息
	config, err := loadConfig(configPath)
	if err != nil {
		return nil, err
	}
	//启动过期删除
	curNode := expirationDelete.Node{Address: "localhost:" + strconv.Itoa(config.Port)}
	ed := expirationDelete.NewExpirationDelete(config.ED, config.Gossip, curNode)
	//创建数据库，并读取预加载数据
	db := skiplist.NewSkipList()
	if err = storage.LoadData(db, ed, config.Database); err != nil {
		log.Println(err)
	}
	//配置节点信息和gossip，创建Gin HTTP路由引擎实例
	currentNode := gossip.Node{Address: "localhost:" + strconv.Itoa(config.Port)}
	newGossip := gossip.NewGossip(config.Gossip, currentNode, db)
	router := gin.Default()
	api.RegisterRoutes(router, db, newGossip, ed) //向路由引擎注册API路由，传入路由引擎、数据库实例和Gossip实例。
	return &Node{
		port:   config.Port,
		config: config,
		db:     db,
		gossip: newGossip,
		router: router,
	}, nil
}

// Start ：启动节点的Gossip服务和http服务
func (node *Node) Start() {
	go node.gossip.Start()
	node.router.Run(":" + strconv.Itoa(node.port))
}

// 读取配置
func loadConfig(path string) (Config, error) {
	var config Config
	data, err := os.ReadFile(path)
	if err != nil {
		return config, err
	}
	err = yaml.Unmarshal(data, &config)
	return config, err
}
