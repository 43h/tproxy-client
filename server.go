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
		LOGE("Error listening:", err)
		return false
	}

	file, err := tmpListener.(*net.TCPListener).File()
	if err != nil {
		LOGE("Error getting file descriptor:", err)
		return false
	}
	fd := int(file.Fd())

	err = syscall.SetsockoptInt(fd, syscall.SOL_IP, syscall.IP_TRANSPARENT, 1)
	if err != nil {
		LOGE("Error setting IP_TRANSPARENT:", err)
		return false
	}
	listener = tmpListener
	return true
}

func closeServer() {
	if listener != nil {
		err := listener.Close()
		if err != nil {
			LOGE("Error closing listener:", err)
		} else {
			LOGI("Server closed")
		}
	} else {
		LOGI("Server closed(skip)")
	}
}

func startServer() {
	LOGI("Server started Listening on ", ConfigParam.Listen)

	for {
		conn, err := listener.Accept()
		if err != nil {
			LOGE("Error accepting:", err)
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
			ip := net.IP(addr.Addr[:]).String()
			port := addr.Port
			LOGI("rcv connect from IPv4 Address: %s, Port: %d\n", ip, port)
			return ip + ":" + strconv.Itoa(port), nil
		//case *unix.SockaddrInet6:
		//	ip := net.IP(addr.Addr[:]).String()
		//	port := addr.Port
		//	fmt.Printf("IPv6 Address: %s, Port: %d\n", ip, port)

		//case *unix.SockaddrUnix:
		//	fmt.Printf("Unix Socket Path: %s\n", addr.Name)

		default:
		}
	}
	return "", errors.New("Unknown address type")
}

func handleRequest(conn net.Conn) {
	LOGI("New connection accepted")
	ipstr, err := getOriginalDst(conn.(*net.TCPConn))
	if err != nil || ipstr == "" {
		err := conn.Close()
		if err != nil {
			LOGE("Error closing new connection:", err)
			return
		}
	}
	connID := uuid.New().String()
	LOGI("new Connection ID: %s\n", connID)

	clientAddEventConnect(connID, ipstr, conn)

	buf := make([]byte, 10240)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			LOGE("Error reading, ", connID, " ", err)
			conn.Close()
			clientAddEventDisconnect(connID)
			return
		} else {
			LOGI("Read from client:", connID, " length:", n)
			clientAddEventMsg(connID, buf[:n], n)
		}
	}
}
