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

func initProxy() bool {
	tmpListener, err := net.Listen("tcp", ConfigParam.Listen)
	if err != nil {
		LOGE("fail to listening ", err)
		return false
	}

	file, err := tmpListener.(*net.TCPListener).File()
	if err != nil {
		LOGE("fail to get file descriptor:", err)
		return false
	}
	fd := int(file.Fd())

	err = syscall.SetsockoptInt(fd, syscall.SOL_IP, syscall.IP_TRANSPARENT, 1)
	if err != nil {
		LOGE("fail to set IP_TRANSPARENT:", err)
		return false
	}
	listener = tmpListener
	return true
}

func closeProxy() {
	if listener != nil {
		err := listener.Close()
		if err != nil {
			LOGE("fail to closing listener ", err)
		} else {
			LOGI("Proxy closed")
		}
	} else {
		LOGI("Proxy closed(NULL)")
	}
}

func startProxy() {
	LOGI("Proxy started Listening on ", ConfigParam.Listen)

	for {
		conn, err := listener.Accept()
		if err != nil {
			LOGE("fail to accepting ", err)
			continue
		}

		go handleRequest(conn)
	}
}

func handleRequest(conn net.Conn) {
	LOGI("New connection accepted")
	ipstr, err := getOriginalDst(conn.(*net.TCPConn))
	if err != nil || ipstr == "" {
		LOGE("fail to get dest ip")
		err := conn.Close()
		if err != nil {
			LOGE("fail to closing new connection:", err)
			return
		}
	}
	connID := uuid.New().String()
	LOGI(connID, " new connection")

	AddEventConnect(connID, ipstr, conn)

	buf := make([]byte, 10240)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			LOGE(connID, " fail to read data ", err)
			conn.Close()
			AddEventDisconnect(connID)
			return
		} else {
			LOGI(connID, " Read from client length:", n)
			AddEventMsg(connID, buf[:n], n)
		}
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
			LOGI("rcv connect from IPv4 Address:", ip, "Port:", port)
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
