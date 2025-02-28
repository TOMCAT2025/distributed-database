package skiplist

import (
	"math/rand"
	"sync"
)

const (
	MaxLevel    = 16
	Probability = 0.5
)

type Student struct {
	ID      int            `json:"id" gorm:"primaryKey"`
	Name    string         `json:"name"`
	Gender  string         `json:"gender"`
	Class   string         `json:"class"`
	Scores  map[string]int `json:"scores" gorm:"serializer:json"`
	Version int64          `json:"version"`
	Deleted bool           `json:"deleted" gorm:"default:false"` //删除标识（是否被删除）
}

type Node struct {
	Student Student
	Next    []*Node
}

type SkipList struct {
	head  *Node
	level int
	mutex sync.RWMutex
}

func NewSkipList() *SkipList {
	return &SkipList{
		head:  &Node{Next: make([]*Node, MaxLevel)},
		level: 1,
	}
}

// 随机节点层数
func (sl *SkipList) randomLevel() int {
	level := 1
	for rand.Float64() < Probability && level < MaxLevel {
		level++
	}
	return level
}

// 插入节点
func (sl *SkipList) Insert(student Student) {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()

	//随机出节点层数
	level := sl.randomLevel()
	if level > sl.level {
		sl.level = level
	}
	node := &Node{Student: student, Next: make([]*Node, level)} //新节点
	current := sl.head
	update := make([]*Node, level) //存放插入节点在每一层所对应的前一个节点

	for i := level - 1; i >= 0; i-- {
		for current.Next[i] != nil && current.Next[i].Student.ID < student.ID {
			current = current.Next[i]
		}
		update[i] = current
	}

	for i := 0; i < level; i++ {
		node.Next[i] = update[i].Next[i]
		update[i].Next[i] = node
	}
}

// 搜索节点
func (sl *SkipList) Search(id int) *Student {
	sl.mutex.RLock()
	defer sl.mutex.RUnlock()

	current := sl.head
	for i := sl.level - 1; i >= 0; i-- {
		for current.Next[i] != nil && current.Next[i].Student.ID < id {
			current = current.Next[i]
		}
	}
	current = current.Next[0]
	if current != nil && current.Student.ID == id {
		return &current.Student
	}
	return nil
}

// 删除节点
func (sl *SkipList) Delete(id int) {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()

	current := sl.head
	update := make([]*Node, sl.level)
	for i := sl.level - 1; i >= 0; i-- {
		for current.Next[i] != nil && current.Next[i].Student.ID < id {
			current = current.Next[i]
		}
		update[i] = current
	}

	current = current.Next[0]
	if current != nil && current.Student.ID == id {
		for i := 0; i < sl.level; i++ {
			if update[i].Next[i] != current {
				break
			}
			update[i].Next[i] = current.Next[i]
		}
		for sl.level > 1 && sl.head.Next[sl.level-1] == nil {
			sl.level--
		}
	}
}

// 更新节点信息
func (sl *SkipList) Update(student Student) {
	sl.Delete(student.ID)
	sl.Insert(student)
}

// 统计节点个数
func (sl *SkipList) Count() int {
	sl.mutex.RLock()
	defer sl.mutex.RUnlock()

	count := 0
	current := sl.head.Next[0]
	for current != nil {
		count++
		current = current.Next[0]
	}
	return count
}

// 返回第一个节点（）
func (sl *SkipList) Head() *Node {
	return sl.head.Next[0]
}
