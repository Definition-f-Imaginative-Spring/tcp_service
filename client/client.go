package main

import (
	"bufio"
	"fmt"
	"github.com/Definition-f-Imaginative-Spring/tep_service/server/connection"
	"net"
	"os"
	"strings"
)

var running = true

func main() {
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		fmt.Println("err :", err)
		return
	}
	defer CloseConn(conn)

	fmt.Println("欢迎来到Chat系统")
	fmt.Println("请先注册")
	inputReader := bufio.NewReader(os.Stdin)
	SetName(conn, inputReader)

	for {
		fmt.Println(".....主界面.....")
		fmt.Println("-1.进入公共聊天室-")
		fmt.Println("2.展示所有连接用户")
		fmt.Println("---3.用户私聊---")
		fmt.Println("---4.退出系统---")
		fmt.Print("-请输入您的操作- ")
		input, errR := inputReader.ReadString('\n')
		if errR != nil {
			fmt.Println("errR :", errR)
			continue
		}
		input = strings.TrimSpace(input)
		switch input {
		case "1":
			fmt.Println("可以开始聊天了,输入quit/QUIT退出聊天室")
			go Receive(conn)
			for {
				if Write(conn, inputReader) {
					break
				}
			}
		case "2":
			fmt.Println("在公共聊天室输入LIST即可查看所有用户")
		case "3":
			fmt.Println("输入[private]username:message的格式即可私聊给对方")
		case "4":
			fmt.Println("正在退出")
			return
		default:
			fmt.Println("无效输入")
		}
	}
}

// SetName 设置用户名
func SetName(conn net.Conn, inputReader *bufio.Reader) {
	for {
		fmt.Println("请输入您想设置的id")
		Write(conn, inputReader)
		result, err := connection.ReadMessage(conn)
		if err != nil {
			return
		}
		if strings.TrimSpace(result) == "PING" {
			err = connection.SendWithPrefix(conn, "PONG")
			if err != nil {
				fmt.Println("err :", err)
			}
			continue
		}

		if strings.TrimSpace(result) == "设置成功" {
			fmt.Println("用户名设置成功！")
			break
		} else {
			fmt.Println("名称重复请重新输入")
		}
	}
}

// CloseConn 关闭连接
func CloseConn(conn net.Conn) {
	err := conn.Close()
	if err != nil {
		fmt.Println("close err :", err)
	}
}

// Write 向服务端写入
func Write(conn net.Conn, inputReader *bufio.Reader) bool {
	input, _ := inputReader.ReadString('\n')
	inputInfo := strings.TrimSpace(input)

	if strings.ToUpper(inputInfo) == "QUIT" {
		running = false
		return true
	}
	err := connection.SendWithPrefix(conn, inputInfo)
	if err != nil {
		return true
	}
	return false
}

// Receive 接收服务端返回的消息并输出
func Receive(conn net.Conn) {
	for running {
		n, err := connection.ReadMessage(conn)
		if err != nil {
			return
		}

		if strings.TrimSpace(string(n)) == "PING" {
			err := connection.SendWithPrefix(conn, "PONG")
			if err != nil {
				fmt.Println("心跳回复失败:", err)
				return
			}
			continue
		}
		fmt.Println(n)
	}
}
