//监听需要代理的客户端请求
package main

import (
	"fmt"
	"golang.org/x/sys/unix"
	"net"
	"os"
	"syscall"
)

// 获取原始目的地址
func getOriginalDst(conn *net.TCPConn) (string, int, error) {
	// 获取底层文件描述符
	file, err := conn.File() // conn.File() 返回一个 *os.File 对象
	if err != nil {
		return "", 0, fmt.Errorf("failed to get file descriptor: %v", err)
	}
	fd := file.Fd() // 获取文件描述符，类型为 uintptr

	sa, err := unix.Getsockname(int(fd))
	if err != nil {
		return "", 0, fmt.Errorf("getsockopt failed: %v", err)
	} else {
		switch addr := sa.(type) {
		case *unix.SockaddrInet4:
			// 打印 IPv4 地址
			ip := net.IP(addr.Addr[:]).String()
			port := addr.Port
			fmt.Printf("IPv4 Address: %s, Port: %d\n", ip, port)

		case *unix.SockaddrInet6:
			// 打印 IPv6 地址
			ip := net.IP(addr.Addr[:]).String()
			port := addr.Port
			fmt.Printf("IPv6 Address: %s, Port: %d\n", ip, port)

		case *unix.SockaddrUnix:
			// 打印 Unix 域套接字地址
			fmt.Printf("Unix Socket Path: %s\n", addr.Name)

		default:
			fmt.Println("Unknown address type")
		}
	}
	return "123", 10, nil
}

func Server() {
	// 创建一个监听器
	listener, err := net.Listen("tcp", "192.168.2.3:80")
	if err != nil {
		fmt.Println("Error listening:", err)
		os.Exit(1)
	}
	defer listener.Close()

	// 获取文件描述符
	file, err := listener.(*net.TCPListener).File()
	if err != nil {
		fmt.Println("Error getting file descriptor:", err)
		os.Exit(1)
	}
	fd := int(file.Fd())

	// 设置套接字选项为透明
	err = syscall.SetsockoptInt(fd, syscall.SOL_IP, syscall.IP_TRANSPARENT, 1)
	if err != nil {
		fmt.Println("Error setting IP_TRANSPARENT:", err)
		os.Exit(1)
	}

	fmt.Println("Listening on 0.0.0.0:8080 with IP_TRANSPARENT")

	for {
		// 接受一个新的连接
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting:", err)
			continue
		}

		// 处理连接
		go handleRequest(conn)
	}
}

func handleRequest(conn net.Conn) {
	// 处理连接的逻辑
	fmt.Println("New connection accepted")
	getOriginalDst(conn.(*net.TCPConn))
	// 读取客户端发送的数据
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Println("Error reading:", err)
		return
	}
	fmt.Printf("Received data: %s\n", string(buf[:n]))

	// 向客户端发送响应
	response := "HTTP/1.1 200 OK\r\nContent-Length: 13\r\n\r\nHello, World!"
	_, err = conn.Write([]byte(response))
	if err != nil {
		fmt.Println("Error writing:", err)
		return
	}
	conn.Close()
}
