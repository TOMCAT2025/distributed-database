package storage

import (
	"distributed-in-memory-db/expirationDelete"
	"distributed-in-memory-db/skiplist"
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"strconv"
)

type DBConfig struct {
	Type     string `yaml:"type"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
}

func LoadData(db *skiplist.SkipList, ed *expirationDelete.ExpirationDelete, config DBConfig) error {
	//创建数据源
	dsn := config.User + ":" + config.Password + "@tcp(" + config.Host + ":" + strconv.Itoa(config.Port) + ")/" + config.DBName + "?charset=utf8mb4&parseTime=True&loc=Local"
	//连接数据库并检查是否发生错误
	gormDB, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %v", err)
	}

	//使用GORM 的 Find 方法查询 students 表，并将结果填充到 students 切片中
	var students []skiplist.Student
	result := gormDB.Find(&students)
	if result.Error != nil {
		return fmt.Errorf("failed to query students table: %v", result.Error)
	}

	//1.遍历 students 切片中的每个学生，并输出到控制台
	//2.将当前遍历到的学生数据插入到跳表中。并开启定时删除
	fmt.Printf("Loaded %d students from database\n", len(students))
	for _, student := range students {
		fmt.Printf("Loading student: ID=%d, Name=%s, Class=%s, Version=%d\n",
			student.ID, student.Name, student.Class, student.Version)
		db.Insert(student)
		go func() {
			err := ed.ExpirationDeleteStart(db, student.ID)
			if err != nil {
				fmt.Println(err.Error())
			}
		}()
	}
	return nil
}
