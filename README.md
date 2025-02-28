# EchoDB食用指南

## 项目目标
使用 Go 语言实现一个分布式内存数据库。数据库采用 NoSQL 类型，支持相同实例下的多实例业务。

## 项目内容
### 数据结构
- 跳表：跳表结构体的 value 为学生结构体（包含学号，姓名，性别，班级，和成绩（map 存储，键为 string 类型的课程名称，值为分数））。
- 基本功能实现：
  1. 插入
  2. 删除
  3. 更新
  4. 查询
  5. 统计节点个数

### 线程模型
使用多线程的锁模型(RWMutex)保证添加和修改学生信息的原子性

### 数据一致性
使用最终一致性 Gossip 协议进行节点间的数据同步。
Gossip同步的触发通过select和channel实现，触发有两种方式  
                        1.定时器触发，间隔时间为Interval `yaml:"interval"`  
                        2.监听g.syncChan通道触发（立即同步通过该途径实现）  
                        当外部用户通过API进行了增删查改操作时，会调用gossip包下的TriggerSync()函数，向syncChan发一个空结构体触发立即同步  
同步操作中加了一个摘要，对比出需要同步的数据，只对这一部分数据同步，可以减少对内存的写操作，提升速度

### 数据源
使用 GORM，在启动时调用storage.go中的LoadData方法从外部 MySQL 中读取预加载数据。

#### 前期准备：数据库配置（为预加载准备数据）
1. 创建数据库：
```sql
DROP DATABASE IF EXISTS students;
CREATE DATABASE students;
USE students;
CREATE TABLE students (
                        id INT PRIMARY KEY,
                        name VARCHAR(100) NOT NULL,
                        gender VARCHAR(10),
                        class VARCHAR(50),
                        scores TEXT,
                        version BIGINT,
                        deleted BOOLEAN DEFAULT FALSE
) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

INSERT INTO students (id, name, gender, class, scores, version, deleted)
VALUES
  (1, 'Alice', 'Female', '1A', '{"Math": 90, "English": 85}', UNIX_TIMESTAMP()*1000000000, false),
  (2, 'Bob', 'Male', '1A', '{"Math": 85, "English": 90}', UNIX_TIMESTAMP()*1000000000, false),
  (3, 'Charlie', 'Male', '1B', '{"Math": 95, "English": 88}', UNIX_TIMESTAMP()*1000000000, false);
```
### 通信协议
一致性算法中各个节点的内部通信使用 HTTP 协议，对暴露给用户的外部接口也使用 HTTP 协议。

### 业务功能
使用 Gin 框架实现了内存数据库的增删查改功能。

### 可插拔式配置
编写一个配置文件yaml在项目运行时进行配置。

### 过期删除策略
采用定时删除的方法实现过期删除，在配置文件中（expiration_time）可以设置过期时间。
对每条数据，在被插入后新开一个goroutine进行延时任务，到时间后删除该数据

### 其他说明
删除操作，外部用户通过api进行的删除操作为假删除，过期删除策略使用的是真删除

## 食用方法
1. 下载项目代码。
2. 配置 `config.yaml` 文件
3. 运行项目：
   ```sh
    # 启动第一个节点（端口 8080）
    go run main.go --config config.yaml

   ```
   修改config.yaml的port为8081
   ```sh
    # 启动第二个节点（端口 8081）
    go run main.go --config config.yaml
   ```
4. 使用 API 进行数据操作。

## API
### 插入学生数据
- 方法：POST
- 路径：`/AddStudent`
- 请求体：
  ```json
  {
    "ID": 4,
    "Name": "Kd",
    "Gender": "Female",
    "Class": "4A",
    "Scores": {"Math": 90, "Science": 95}
  }
  ```
- 响应：
  ```json
  {
    "message": "student inserted"
  }
  ```

### 查询学生数据
- 方法：GET
- 路径：`/QueryStudent?id=4`
- 响应：
  ```json
  {
    "ID": 4,
    "Name": "Kd",
    "Gender": "Female",
    "Class": "1A",
    "Scores": {"Math": 90, "Science": 95}
  }
  ```

### 删除学生数据
- 方法：DELETE
- 路径：`/DeleteStudent?id=4`
- 响应：
  ```json
  {
    "message": "student deleted"
  }
  ```

### 更新学生数据
- 方法：GET
- 路径：`/UpdateStudent?id=4`
- 请求体：
  ```json
  {
    "ID": 4,
    "Name": "Alice",
    "Gender": "Female",
    "Class": "1A",
    "Scores": {"Math": 90, "Science": 95}
  }
  ```
- 响应：
  ```json
  {
    "message": "student updated"
  }
  ```



