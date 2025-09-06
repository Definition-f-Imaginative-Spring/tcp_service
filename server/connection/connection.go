package connection

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
)

type ConnectManager struct {
	Connections map[net.Conn]bool
	Mutex       sync.Mutex
	connToUser  map[net.Conn]string
}

// NewConnectionManager 构造函数
func NewConnectionManager() *ConnectManager {
	return &ConnectManager{
		Connections: make(map[net.Conn]bool),
		connToUser:  make(map[net.Conn]string),
	}
}

// SetupName 设置ID
func (cm *ConnectManager) SetupName(conn net.Conn) string {
	username := conn.RemoteAddr().String()
	reader := bufio.NewReader(conn)

	for {
		var boolean = true

		username, _ = cm.Read(conn, reader)

		cm.Mutex.Lock()
		for v := range cm.connToUser {
			if username == cm.connToUser[v] {
				boolean = false
				_, err := conn.Write([]byte("名称重复，请重新输入\n"))
				if err != nil {
					return ""
				}
				break
			}
		}
		cm.Mutex.Unlock()
		if boolean {
			_, err := conn.Write([]byte("设置成功"))
			if err != nil {
				return ""
			}
			break
		}
	}
	cm.Mutex.Lock()
	cm.Connections[conn] = true
	cm.connToUser[conn] = username
	cm.Mutex.Unlock()
	fmt.Printf("用户%s 用户地址%v连接成功\n", username, conn.RemoteAddr())
	cm.Broadcast([]byte(fmt.Sprintf("[系统] %s 加入了聊天室\n", username)))
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

// Read 从客户端读取
func (cm *ConnectManager) Read(conn net.Conn, reader *bufio.Reader) (string, bool) {
	var buf [4096]byte
	n, errR := reader.Read(buf[:])
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
	return string(buf[:n]), true
}

// close 关闭连接
func (cm *ConnectManager) close(conn net.Conn) {
	if _, exists := cm.Connections[conn]; !exists {
		return
	}
	cm.Mutex.Lock()
	delete(cm.Connections, conn)
	delete(cm.connToUser, conn)
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
	reader := bufio.NewReader(conn)
	for {
		result, b := cm.Read(conn, reader)
		if !b {
			break
		}
		//去除首位空白符
		trimmedResult := strings.TrimSpace(result)

		if trimmedResult == "LIST" {
			cm.handleListCommand(conn)
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
					_, err := conn.Write([]byte("[系统] 私聊格式错误！正确格式：[private]目标用户名:消息内容"))
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
	_, err := conn.Write([]byte("在线用户列表:\n"))
	if err != nil {
		fmt.Println("发送列表标题失败:", err)
		return
	}
	cm.Mutex.Lock()
	defer cm.Mutex.Unlock()

	for c, user := range cm.connToUser {
		if cm.Connections[c] {
			userInfo := fmt.Sprintf("- %s (%s)\n", user, c.RemoteAddr().String())
			_, err := conn.Write([]byte(userInfo))
			if err != nil {
				fmt.Println("发送用户信息失败:", err)
				continue
			}
		}
	}
	_, err = conn.Write([]byte("列表结束\n"))
	if err != nil {
		fmt.Println("发送列表结束标记失败:", err)
	}
}

// Process 进程
func (cm *ConnectManager) Process(conn net.Conn) {
	defer cm.close(conn)

	username := cm.SetupName(conn)

	cm.SendMessage(conn, username)
}

// Broadcast 广播形式发送信息到客户端
func (cm *ConnectManager) Broadcast(message []byte) {
	defer cm.Mutex.Unlock()
	cm.Mutex.Lock()
	for conn := range cm.Connections {
		_, errW := conn.Write(message)
		if errW != nil {
			fmt.Println("broad write err", errW)
			cm.close(conn)
		}
	}
}

// Private 发送到个人
func (cm *ConnectManager) Private(message []byte, s string) {
	defer cm.Mutex.Unlock()
	cm.Mutex.Lock()
	for conn := range cm.connToUser {
		if cm.connToUser[conn] == s {
			_, errW := conn.Write(message)
			if errW != nil {
				fmt.Println("broad write err", errW)
				cm.close(conn)
			}
		}

	}
}
