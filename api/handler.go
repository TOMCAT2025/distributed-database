package api

import (
	"distributed-in-memory-db/expirationDelete"
	//"distributed-in-memory-db/expirationDelete"
	"distributed-in-memory-db/gossip"
	"distributed-in-memory-db/skiplist"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"time"
)

func RegisterRoutes(router *gin.Engine, db *skiplist.SkipList, sync *gossip.Gossip, ed *expirationDelete.ExpirationDelete) {
	// AddStudent 接口
	router.POST("/AddStudent", func(c *gin.Context) {
		var student skiplist.Student
		if err := c.ShouldBindJSON(&student); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 检查是否已存在
		existingStudent := db.Search(student.ID)
		if existingStudent != nil && !existingStudent.Deleted {
			c.JSON(http.StatusConflict, gin.H{"error": "student already exists"})
			return
		}

		// 设置初始版本号
		student.Version = time.Now().UnixNano()
		db.Insert(student)
		//新开一个线程执行过期删除操作
		go func() {
			err := ed.ExpirationDeleteStart(db, student.ID)
			if err != nil {
				fmt.Println(err.Error())
			}
		}()
		// 触发同步
		sync.TriggerSync()
		c.JSON(http.StatusOK, gin.H{"message": "student inserted"})
	})

	//DeleteStudent，删除学生接口（假删除）
	router.DELETE("/DeleteStudent", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Query("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID"})
			return
		}
		student := db.Search(id)
		if student == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "student not found"})
			return
		}
		// 标记为删除并更新版本号
		student.Deleted = true
		student.Version = time.Now().UnixNano()
		db.Update(*student)
		// 触发同步
		sync.TriggerSync()
		c.JSON(http.StatusOK, gin.H{"message": "student deleted"})
	})

	// UpdateStudent 更新学生信息接口
	router.POST("/UpdateStudent", func(c *gin.Context) {
		var student skiplist.Student
		if err := c.ShouldBindJSON(&student); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		existingStudent := db.Search(student.ID)
		if existingStudent != nil {
			// 更新时增加版本号
			student.Version = time.Now().UnixNano()
			db.Delete(student.ID)
			db.Insert(student)
		} else {
			// 如果是新记录，设置初始版本号
			student.Version = time.Now().UnixNano()
			db.Insert(student)
		}
		// 触发同步
		sync.TriggerSync()
		c.JSON(http.StatusOK, gin.H{"message": "student updated"})
	})

	router.GET("/QueryStudent", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Query("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID, " + err.Error()})
			return
		}

		student := db.Search(id)
		if student == nil || student.Deleted {
			c.JSON(http.StatusNotFound, gin.H{"error": "student not found"})
			return
		}
		c.JSON(http.StatusOK, student)
	})

	// SyncDigest 处理摘要同步接口
	router.POST("/SyncDigest", func(c *gin.Context) {
		var syncData gossip.SyncData
		if err := c.ShouldBindJSON(&syncData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		sync.ReceiveDigest(syncData)
		c.JSON(http.StatusOK, gin.H{"message": "digest processed"})
	})

	// RequestData 处理数据请求接口
	router.POST("/RequestData", func(c *gin.Context) {
		var request struct {
			IDs []int `json:"ids"`
		}
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 返回请求的数据
		response := make([]skiplist.Student, 0)
		for _, id := range request.IDs {
			if student := db.Search(id); student != nil {
				response = append(response, *student)
			}
		}
		c.JSON(http.StatusOK, response)
	})

	//定时删除里同步节点数据的删除接口
	router.GET("/ExpirationDelete", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Query("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID"})
		}
		db.Delete(id)
		c.JSON(http.StatusOK, gin.H{"message": "deleted successfully"})
	})
}
