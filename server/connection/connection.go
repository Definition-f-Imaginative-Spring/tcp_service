package connection

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

type ConnectManager struct {
	Connections map[net.Conn]bool
	Mutex       sync.Mutex
	connToUser  map[net.Conn]string
	lastActive  map[net.Conn]int64
}

// NewConnectionManager 构造函数
func NewConnectionManager() *ConnectManager {
	return &ConnectManager{
		Connections: make(map[net.Conn]bool),
		connToUser:  make(map[net.Conn]string),
		lastActive:  make(map[net.Conn]int64),
	}
}

// SetupName 设置ID
func (cm *ConnectManager) SetupName(conn net.Conn) string {
	var username string

	for {
		inputName, ok := cm.Read(conn)
		if !ok {
			return ""
		}

		// 过滤心跳消息
		trimmed := strings.TrimSpace(inputName)
		if trimmed == "PING" || trimmed == "PONG" {
			continue
		}

		username = trimmed
		// 检查用户名是否唯一
		isUnique := true

		cm.Mutex.Lock()
		for _, v := range cm.connToUser {
			if username == v {
				isUnique = false
				break
			}
		}
		cm.Mutex.Unlock()

		if !isUnique {
			if err := SendWithPrefix(conn, "名称重复，请重新输入\n"); err != nil {
				return ""
			}
			continue
		}
		if err := SendWithPrefix(conn, "设置成功"); err != nil {
			return ""
		}
		break
	}

	// 将连接加入管理器
	cm.Mutex.Lock()
	cm.Connections[conn] = true
	cm.connToUser[conn] = username
	cm.lastActive[conn] = time.Now().Unix()
	cm.Mutex.Unlock()

	fmt.Printf("用户%s 用户地址%v连接成功\n", username, conn.RemoteAddr())
	return username
}

// Listen 启动监听
func (cm *ConnectManager) Listen() {
	Listen, errL := net.Listen("tcp", "localhost:8080")
	if errL != nil {
		fmt.Println("Listener :", errL)
		return
	}
	fmt.Println("监听已启动，等待连接接入")
	go cm.heartbeatCheck()

	defer func(Listen net.Listener) {
		errLC := Listen.Close()
		if errLC != nil {
			fmt.Println("Error closing listener")
		}
	}(Listen)

	for {
		conn, err := Listen.Accept()
		if err != nil {
			fmt.Println("connect :", err)
		}
		go cm.Process(conn)
	}
}

// heartbeatCheck 心跳检测
func (cm *ConnectManager) heartbeatCheck() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now().Unix()

		var toClose []net.Conn
		var toPing []net.Conn

		// 持锁检查状态，收集待操作的连接
		cm.Mutex.Lock()
		for conn := range cm.Connections {
			last, ok := cm.lastActive[conn]
			if !ok || now-last > 30 {
				fmt.Printf("用户 %s 超时未响应，准备断开连接\n", cm.connToUser[conn])
				toClose = append(toClose, conn)
			} else {
				toPing = append(toPing, conn)
			}
		}
		cm.Mutex.Unlock()

		// 先发送心跳
		for _, conn := range toPing {
			if err := SendWithPrefix(conn, "PING"); err != nil {
				fmt.Println("发送心跳失败:", err)
				toClose = append(toClose, conn)
			}
		}

		// 关闭超时或发送失败的连接
		for _, conn := range toClose {
			cm.close(conn) // 这里已经带锁删除 map
		}
	}
}

// Read 从客户端读取
func (cm *ConnectManager) Read(conn net.Conn) (string, bool) {
	n, errR := ReadMessage(conn)
	if errR != nil {
		if errR == io.EOF {
			fmt.Println("客户端断开连接")
		} else if strings.Contains(errR.Error(), "forcibly closed") {
			fmt.Println("客户端断开连接")
		} else {
			fmt.Println("read err", errR)
		}
		cm.close(conn)
		return "", false
	}

	trimmed := strings.TrimSpace(n)
	if trimmed == "PING" || trimmed == "PONG" {
		cm.Mutex.Lock()
		cm.lastActive[conn] = time.Now().Unix()
		cm.Mutex.Unlock()
		return "", true
	}

	cm.Mutex.Lock()
	cm.lastActive[conn] = time.Now().Unix()
	cm.Mutex.Unlock()

	return n, true
}

// close 关闭连接
func (cm *ConnectManager) close(conn net.Conn) {
	if _, exists := cm.Connections[conn]; !exists {
		return
	}
	cm.Mutex.Lock()
	delete(cm.Connections, conn)
	delete(cm.connToUser, conn)
	delete(cm.lastActive, conn)
	cm.Mutex.Unlock()

	errC := conn.Close()
	if errC != nil {
		fmt.Println("Error closing connection")
	} else {
		fmt.Println("Closing connection SUCCESS")
	}
}

// SendMessage 把消息送往聊天室
func (cm *ConnectManager) SendMessage(conn net.Conn, username string) {
	for {
		result, b := cm.Read(conn)
		if !b {
			break
		}
		//去除首位空白符
		trimmedResult := strings.TrimSpace(result)

		if trimmedResult == "" {
			continue
		}

		if trimmedResult == "LIST" {
			cm.handleListCommand(conn)
			continue
		}

		if trimmedResult == "PONG" {
			continue
		}

		message := fmt.Sprintf("用户%s:%s \n", username, result)
		if len(result) <= 9 {
			fmt.Printf("用户%s:%s \n", username, result)
			cm.Broadcast([]byte(message))
		} else {
			if result[:9] != "[private]" {
				fmt.Printf("用户%s:%s \n", username, result)
				cm.Broadcast([]byte(message))
			} else {
				message = result[9:]
				parts := strings.SplitN(message, ":", 2)
				if len(parts) < 2 {
					err := SendWithPrefix(conn, "\"[系统] 私聊格式错误！正确格式：[private]目标用户名:消息内容\"")
					if err != nil {
						return
					}
					continue
				}
				username2 := parts[0]
				message := parts[1]
				boolean := true
				for _, v := range cm.connToUser {
					if v == username2 {
						fmt.Printf("用户%s私聊%s：%s\n", username, username2, message)
						cm.Private([]byte(fmt.Sprintf("来自用户%s的私聊内容:%s", username, message)), username2)
						boolean = false
					}
				}
				if boolean {
					fmt.Println("不存在该用户")
					cm.Private([]byte("该用户不存在"), username)
					continue
				}
			}
		}

	}
}

// handleListCommand 处理LIST命令，向客户端发送在线用户列表
func (cm *ConnectManager) handleListCommand(conn net.Conn) {
	err := SendWithPrefix(conn, "在线用户列表:\n")
	if err != nil {
		fmt.Println("标题发送失败")
	}

	cm.Mutex.Lock()
	users := make([]string, 0, len(cm.connToUser))
	for c, user := range cm.connToUser {
		if cm.Connections[c] {
			users = append(users, fmt.Sprintf("- %s (%s)\n", user, c.RemoteAddr().String()))
		}
	}
	cm.Mutex.Unlock()

	for _, userInfo := range users {
		err = SendWithPrefix(conn, userInfo)
		if err != nil {
			fmt.Println("发送信息失败")
		}
	}
	_ = SendWithPrefix(conn, "列表结束\n")
}

// Process 进程
func (cm *ConnectManager) Process(conn net.Conn) {
	defer cm.close(conn)

	username := cm.SetupName(conn)

	cm.SendMessage(conn, username)
}

// Broadcast 广播形式发送信息到客户端
func (cm *ConnectManager) Broadcast(message []byte) {

	cm.Mutex.Lock()
	users := make([]net.Conn, 0, len(cm.Connections))
	for conn := range cm.Connections {
		users = append(users, conn)
	}
	cm.Mutex.Unlock()

	for _, u := range users {
		errW := SendWithPrefix(u, string(message))
		if errW != nil {
			fmt.Println("broad write err", errW)
			cm.close(u)
		}
	}
}

// Private 发送到个人
func (cm *ConnectManager) Private(message []byte, s string) {

	cm.Mutex.Lock()
	users := make([]net.Conn, 0, len(cm.connToUser))
	for conn := range cm.connToUser {
		if cm.connToUser[conn] == s {
			users = append(users, conn)
			break
		}
	}
	cm.Mutex.Unlock()

	for _, conn := range users {
		errW := SendWithPrefix(conn, string(message))
		if errW != nil {
			fmt.Println("private write err", errW)
			cm.close(conn)
		}
	}

}

// SendWithPrefix 加前缀发送
func SendWithPrefix(conn net.Conn, msg string) error {
	data := []byte(msg)
	length := uint32(len(data))
	buf := new(bytes.Buffer)

	if err := binary.Write(buf, binary.BigEndian, length); err != nil {
		return fmt.Errorf("binary write err: %v", err)
	}

	if _, err := buf.Write(data); err != nil {
		return fmt.Errorf("buffer write err: %v", err)
	}

	if _, err := conn.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("conn write err: %v", err)
	}
	return nil
}

// ReadMessage 读取带前缀的信息
func ReadMessage(conn net.Conn) (string, error) {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(conn, lenBuf); err != nil {
		return "", err
	}
	length := binary.BigEndian.Uint32(lenBuf)

	msgBuf := make([]byte, length)
	if _, err := io.ReadFull(conn, msgBuf); err != nil {
		return "", err
	}

	return string(msgBuf), nil
}
