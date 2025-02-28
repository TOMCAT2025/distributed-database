package gossip

import (
	"bytes"
	"distributed-in-memory-db/skiplist"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

type Node struct {
	Address string `yaml:"address"`
}

type Gossip struct {
	nodes       []Node
	currentNode Node
	db          *skiplist.SkipList
	mutex       sync.RWMutex
	config      Config
	fanout      int
	syncChan    chan struct{} // 添加触发同步的通道
}

// Config :gossip配置信息
type Config struct {
	Interval time.Duration `yaml:"interval"`
	Nodes    []Node        `yaml:"nodes"`
	Fanout   int           `yaml:"fanout"`
}

type SyncData struct {
	Students  []skiplist.Student `json:"students"`
	NodeAddr  string             `json:"node_addr"`
	Round     int64              `json:"round"`
	Timestamp time.Time          `json:"timestamp"`
	Digest    map[int]int64      `json:"digest"`
}

// NewGossip :初始化gossip实例
func NewGossip(config Config, currentNode Node, db *skiplist.SkipList) *Gossip {
	if config.Fanout <= 0 {
		config.Fanout = 3
	}
	return &Gossip{
		nodes:       config.Nodes,
		currentNode: currentNode,
		db:          db,
		config:      config,
		fanout:      config.Fanout,
		syncChan:    make(chan struct{}, 1), // 带缓冲的通道，避免阻塞
	}
}

// TriggerSync :用来触发立即同步的一个方法
func (g *Gossip) TriggerSync() {
	// 非阻塞发送，如果通道已满则跳过
	select {
	case g.syncChan <- struct{}{}:
	default:
	}
}

// Start :用来启动gossip协议的同步过程
// 有两个途径：1.定时器启动，时间间隔Interval可以在yaml中配置
//
//	2.监听g.syncChan启动（立即同步通过该途径实现）
func (g *Gossip) Start() {
	ticker := time.NewTicker(g.config.Interval)
	fmt.Println(g.nodes)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			g.syncRound()
		case <-g.syncChan:
			// 收到触发信号时立即同步
			g.syncRound()
		}
	}
}

// 该方法生成数据库节点的数据摘要，摘要用于数据同步时对比两个节点的数据，找出需要同步的数据
func (g *Gossip) prepareDigest() map[int]int64 {
	g.mutex.RLock() //读锁
	defer g.mutex.RUnlock()

	//创建摘要digest，以学生id为键，版本号为值
	digest := make(map[int]int64)
	//获取第一个节点
	current := g.db.Head()
	for current != nil {
		digest[current.Student.ID] = current.Student.Version
		current = current.Next[0]
	}
	return digest
}

// 数据同步函数，在列表节点中随机选择fanout个节点（fanout可在yaml中配置），向其发送数据摘要
func (g *Gossip) syncRound() {
	//随机选择节点
	targetNodes := g.selectRandomNodes(g.fanout)
	if len(targetNodes) == 0 {
		return
	}

	var wg sync.WaitGroup
	for _, node := range targetNodes {
		//若目标节点为当前节点，跳过不发送
		if node.Address == g.currentNode.Address {
			continue
		}
		wg.Add(1)
		//新开goroutine发送摘要
		go func(n Node) {
			defer wg.Done()
			// 发送摘要
			digest := g.prepareDigest()
			syncData := SyncData{
				NodeAddr:  g.currentNode.Address,
				Round:     time.Now().UnixNano(),
				Timestamp: time.Now(),
				Digest:    digest,
			}
			if err := g.sendDigest(n, syncData); err != nil {
				log.Printf("Failed to send digest to node %s: %v", n.Address, err)
			}
		}(node)
	}
	wg.Wait()
}

// 根据传入的数字随机选择节点，存到一个Node类型的切片并返回
func (g *Gossip) selectRandomNodes(count int) []Node {
	//如果传入的数字大于等于整个节点列表的总节点数，就把节点列表的节点都返回
	if len(g.nodes) <= count {
		return g.nodes
	}

	indices := make(map[int]bool) //用来记录节点是否被选中
	selected := make([]Node, 0, count)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	//随机出节点索引，并添加到selected中，直到满足数量要求
	for len(selected) < count {
		idx := r.Intn(len(g.nodes))
		if !indices[idx] {
			indices[idx] = true
			selected = append(selected, g.nodes[idx])
		}
	}
	return selected
}

func (g *Gossip) sendDigest(targetNode Node, data SyncData) error {
	//data封装为json格式，存到jsonData里
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	//通过http把摘要发送到目标节点
	resp, err := http.Post(
		"http://"+targetNode.Address+"/SyncDigest",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println("Failed to close body" + err.Error())
		}
	}(resp.Body)
	//检查状态码，不是200返回错误
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return nil
}

func (g *Gossip) ReceiveDigest(data SyncData) {
	//检查接收到的摘要节点地址是否与当前节点地址相同。相同，说明摘要数据是自己发给自己的，直接返回。
	if data.NodeAddr == g.currentNode.Address {
		return
	}

	needSync := make([]int, 0)       //存储需要同步的数据的ID
	localDigest := g.prepareDigest() //生成本地摘要

	// 比较摘要，找出需要同步的数据
	for id, remoteVersion := range data.Digest {
		//needSync存放的是：本地版本小于远程节点的数据（需要更新操作）
		//					本地没有但是远程有的数据(需要)
		localVersion, exists := localDigest[id]
		if !exists || localVersion < remoteVersion {
			needSync = append(needSync, id)
		}
	}

	// 如果有需要同步的数据，发送请求获取数据
	if len(needSync) > 0 {
		g.requestSyncData(data.NodeAddr, needSync)
	}
}

func (g *Gossip) requestSyncData(nodeAddr string, ids []int) {
	//使用匿名结构体把需要同步的数据ID存到request中
	request := struct {
		IDs []int `json:"ids"`
	}{
		IDs: ids,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		log.Printf("Failed to marshal request: %v", err)
		return
	}

	//发送请求，获取数据
	resp, err := http.Post(
		"http://"+nodeAddr+"/RequestData",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		log.Printf("Failed to request data from node %s: %v", nodeAddr, err)
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println("Failed to close body" + err.Error())
		}
	}(resp.Body)

	//解码数据，存放到student切片中
	var students []skiplist.Student
	if err := json.NewDecoder(resp.Body).Decode(&students); err != nil {
		log.Printf("Failed to decode response: %v", err)
		return
	}

	// 更新本地数据
	g.mutex.Lock()
	defer g.mutex.Unlock()

	for _, student := range students {
		existingStudent := g.db.Search(student.ID)
		//远程版本大于本地版本（需要更新，这里即包含有更新操作的数据，也包含了已经在远程被删除的节点，因为用的是假删除，在hander.go中我们的删除操作实际上是一个标记学生删除标识为已删除的更新操作）
		//远程有但是本地有的节点（需要插入，这里使用更新操作代替了插入操作）
		if existingStudent == nil || student.Version > existingStudent.Version {
			if student.Deleted {
				existingStudent.Deleted = true
			} else {
				g.db.Update(student)
			}
		}
	}
}

// GetRequestedData :根据传入的id列表，把本地的学生数据返回
func (g *Gossip) GetRequestedData(ids []int) []skiplist.Student {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	result := make([]skiplist.Student, 0, len(ids))
	for _, id := range ids {
		if student := g.db.Search(id); student != nil {
			result = append(result, *student)
		}
	}
	return result
}
