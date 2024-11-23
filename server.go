//go:build linux
// +build linux

package main

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"syscall"

	"github.com/google/uuid"
	"golang.org/x/sys/unix"
)

var listener net.Listener

func initServer() bool {
	tmpListener, err := net.Listen("tcp", ConfigParam.Listen)
	if err != nil {
		fmt.Println("Error listening:", err)
		return false
	}

	file, err := tmpListener.(*net.TCPListener).File()
	if err != nil {
		fmt.Println("Error getting file descriptor:", err)
	}
	fd := int(file.Fd())

	err = syscall.SetsockoptInt(fd, syscall.SOL_IP, syscall.IP_TRANSPARENT, 1)
	if err != nil {
		fmt.Println("Error setting IP_TRANSPARENT:", err)
	}
	listener = tmpListener
	return true
}

func closeServer() {
	if listener != nil {
		err := listener.Close()
		if err != nil {
			fmt.Println("Error closing listener:", err)
		} else {
			fmt.Println("Server closed")
		}
		fmt.Println("Error closing listener:", err)
	} else {
		fmt.Println("Server closed")
	}
}

func startServer() {
	fmt.Println("Server started")
	fmt.Println("Listening on ", ConfigParam.Listen)

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting:", err)
			continue
		}

		go handleRequest(conn)
	}
}

func getOriginalDst(conn *net.TCPConn) (string, error) {
	file, err := conn.File()
	if err != nil {
		return "", fmt.Errorf("failed to get file descriptor: %v", err)
	}
	fd := file.Fd() // 获取文件描述符，类型为 uintptr

	sa, err := unix.Getsockname(int(fd))
	if err != nil {
		return "", fmt.Errorf("getsockopt failed: %v", err)
	} else {
		switch addr := sa.(type) {
		case *unix.SockaddrInet4:
			// 打印 IPv4 地址
			ip := net.IP(addr.Addr[:]).String()
			port := addr.Port
			fmt.Printf("rcv connect from IPv4 Address: %s, Port: %d\n", ip, port)
			return ip + ":" + strconv.Itoa(port), nil
		//case *unix.SockaddrInet6:
		//	// 打印 IPv6 地址
		//	ip := net.IP(addr.Addr[:]).String()
		//	port := addr.Port
		//	fmt.Printf("IPv6 Address: %s, Port: %d\n", ip, port)

		//case *unix.SockaddrUnix:
		//	// 打印 Unix 域套接字地址
		//	fmt.Printf("Unix Socket Path: %s\n", addr.Name)

		default:
			fmt.Println("Unknown address type")
		}
	}
	return "", errors.New("Unknown address type")
}

func handleRequest(conn net.Conn) {
	// 处理连接的逻辑
	fmt.Println("New connection accepted")
	ipstr, err := getOriginalDst(conn.(*net.TCPConn))
	if err != nil || ipstr == "" {
		err := conn.Close()
		if err != nil {
			fmt.Println("Error closing connection:", err)
			return
		}
	}
	connID := uuid.New().String()
	fmt.Printf("Connection ID: %s\n", connID)

	clientAddEventConnect(connID, ipstr, conn)
	// 读取客户端发送的数据
	buf := make([]byte, 2048)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			fmt.Println("Error reading:", err)
			conn.Close()
			clientAddEventDisconnect(connID)
			return
		} else {
			clientAddEventMsg(connID, buf, n)
		}
	}

}
