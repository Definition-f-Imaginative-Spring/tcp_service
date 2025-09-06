package main

import (
	"bufio"
	"fmt"
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
			go Receive(conn)
			fmt.Println("可以开始聊天了,输入quit/QUIT退出聊天室")
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
			for i := 0; i < 10; i++ {
				fmt.Println("正在退出")
			}

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
		buf := [512]byte{}
		n, err := conn.Read(buf[:])
		if err != nil {
			return
		}
		result := string(buf[:n])
		if strings.TrimSpace(result) == "设置成功" {
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
	inputInfo := strings.Trim(input, "\r\n")

	if strings.ToUpper(inputInfo) == "QUIT" {
		running = false
		return true
	}
	_, err := conn.Write([]byte(inputInfo))
	if err != nil {
		return true
	}
	return false
}

// Receive 接收服务端返回的消息并输出
func Receive(conn net.Conn) {
	buf := [512]byte{}
	for running {
		n, err := conn.Read(buf[:])
		if err != nil {
			return
		}
		fmt.Println(string(buf[:n]))
	}
}
