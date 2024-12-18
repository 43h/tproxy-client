package main

import (
	"errors"
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
		LOGE("proxy listening, fail, ", err)
		return false
	}

	file, err := tmpListener.(*net.TCPListener).File()
	if err != nil {
		LOGE("proxy get file descriptor, fail, ", err)
		return false
	}
	fd := int(file.Fd())

	err = syscall.SetsockoptInt(fd, syscall.SOL_IP, syscall.IP_TRANSPARENT, 1)
	if err != nil {
		LOGE("proxy set IP_TRANSPARENT, fail, ", err)
		return false
	}
	listener = tmpListener
	return true
}

func closeProxy() {
	if listener != nil {
		err := listener.Close()
		if err != nil {
			LOGE("proxy closing listener, fail, ", err)
		} else {
			LOGI("proxy closed")
		}
	} else {
		LOGI("proxy closed(SKIP)")
	}
}

func startProxy() {
	LOGI("Proxy started Listening on ", ConfigParam.Listen)

	for {
		conn, err := listener.Accept()
		if err != nil {
			LOGE("proxy fail to accepting, ", err)
			continue
		} else {
			go handleRequest(conn)
		}
	}
}

func handleRequest(conn net.Conn) {
	LOGD("proxy accept new connection")
	remoteAddr := conn.RemoteAddr().(*net.TCPAddr)
	sourceIP := remoteAddr.IP.String()
	sourcePort := remoteAddr.Port

	ipstr, err := getOriginalDst(conn.(*net.TCPConn))
	if err != nil || ipstr == "" {
		LOGE("proxy get dst ip, fail, ", err)
		err := conn.Close()
		if err != nil {
			LOGE("proxy closing new connection, fail, ", err)
		} else {
			LOGD("proxy closed new connection, success")
		}
		return
	}

	connID := uuid.New().String()
	LOGI(connID, " new connection: ", sourceIP+":"+strconv.Itoa(sourcePort), "---> ", ipstr)

	AddEventConnect(connID, ipstr, conn)

	for {
		buf := make([]byte, 4096)
		n, err := conn.Read(buf)
		if err != nil {
			LOGE(connID, " client--->proxy, read, fail, ", err)
			conn.Close()
			AddEventDisconnect(connID)
			return
		} else {
			LOGD(connID, " client--->proxy, read, success, length: ", n)
			AddEventMsg(connID, buf[:n], n)
		}
	}
}

func getOriginalDst(conn *net.TCPConn) (string, error) {
	file, err := conn.File()
	if err != nil {
		return "", err
	}
	fd := file.Fd() // 获取文件描述符，类型为 uintptr

	sa, err := unix.Getsockname(int(fd))
	if err != nil {
		return "", err
	} else {
		switch addr := sa.(type) {
		case *unix.SockaddrInet4:
			ip := net.IP(addr.Addr[:]).String()
			port := addr.Port
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
