// 代理服务器，监听客户端请求
package main

import (
	"errors"
	"net"
	"strconv"
	"syscall"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sys/unix"
)

var listener net.Listener

func initProxy() bool {
	tmpListener, err := net.Listen("tcp", ConfigParam.Listen) //开始监听
	if err == nil {
		//获取文件描述符
		file, err := tmpListener.(*net.TCPListener).File()
		if err == nil {
			fd := int(file.Fd())
			err = syscall.SetsockoptInt(fd, syscall.SOL_IP, syscall.IP_TRANSPARENT, 1) //设置透明属性
			if err == nil {
				listener = tmpListener
				LOGI("proxy start to listening on " + ConfigParam.Listen + "...")
				return true
			} else {
				LOGE("proxy set IP_TRANSPARENT, fail, ", err)
			}
		} else {
			LOGE("proxy get file descriptor, fail, ", err)
		}
	} else {
		LOGE("proxy listening, fail, ", err)
	}

	return false
}

func closeProxy() {
	if listener != nil {
		err := listener.Close()
		if err == nil {
			LOGI("proxy closed")
		} else {
			LOGE("proxy closing listener, fail, ", err)
		}
	} else {
		LOGI("proxy closed(SKIP)")
	}
}

func startProxy() {
	LOGI("Proxy start to accept new connection ...")

	for {
		conn, err := listener.Accept() //接受客户端连接
		if err == nil {
			go handleNewConnection(conn) //一个协程处理一个客户端
		} else {
			LOGE("proxy fail to accepting, ", err)
			continue
		}
	}
}

func handleNewConnection(conn net.Conn) {
	connUuID := uuid.New().String()                //通过uuid标识客户端连接
	remoteAddr := conn.RemoteAddr().(*net.TCPAddr) //获取源IP和端口信息
	sourceIP := remoteAddr.IP.String()
	sourcePort := remoteAddr.Port
	LOGD(connUuID, " new connection from ", sourceIP, ":", sourcePort)

	realDstIp, err := getOriginalDst(conn.(*net.TCPConn)) //获取真实目的IP和端口信息
	if err != nil || realDstIp == "" {
		LOGE(connUuID, " get dst ip, fail, ", err)
		err := conn.Close()
		if err != nil {
			LOGE(connUuID, " close new connection, fail, ", err)
		} else {
			LOGI(connUuID, " close new connection, success")
		}
		return
	}

	LOGD(connUuID, " new connection: ", sourceIP, ":", sourcePort, "---> ", realDstIp)
	connections[connUuID] = ConnectionInfo{ //记录连接信息
		IPStr:     realDstIp,
		Conn:      conn,
		Timestamp: time.Now().Unix(),
		Status:    Connected,
	}
	connInfo := connections[connUuID]
	AddEventConnect(connUuID, realDstIp) //上报新客户端连接

	for { //接受客户端消息
		buf := make([]byte, 4096)
		err = conn.SetReadDeadline(time.Now().Add(5 * time.Second)) //设置读5秒超时
		if err != nil {
			LOGD(connUuID, " set read deadline, fail, ", err) //Hack: 若设置失败会影响后续主动关闭
		}
		n, err := conn.Read(buf)
		if err == nil {
			AddEventMsg(connUuID, buf[:n], n) //上报客户端数据
			LOGD(connUuID, " client--->proxy, read, success, length: ", n)
		} else {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() { // 处理读取超时
				LOGD(connUuID, " read timeout")
				if connInfo.Status == Connected { //连接正常，继续收取消息
					continue
				} else if connInfo.Status == Disconnect { //上游与真实服务器断开连接，需要主动断开与客户端之间的链接
					LOGI(connUuID, " disconnect connection actively")
				}
			} else {
				LOGE(connUuID, " client--->proxy, read, fail, ", err, ", disconnect connection")
			}

			err = conn.Close() //关闭连接
			if err != nil {
				LOGI(connUuID, " close new connection, fail, ", err)
			} else {
				LOGD(connUuID, " close new connection, success")
			}
			AddEventDisconnect(connUuID) //上报客户端断开事件
			return
		}
	}
}

func getOriginalDst(conn *net.TCPConn) (string, error) { //获取原始请求
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
		//case *unix.SockaddrInet6:  //Todo:支持IPv6
		//	ip := net.IP(addr.Addr[:]).String()
		//	port := addr.Port
		//	fmt.Printf("IPv6 Address: %s, Port: %d\n", ip, port)

		//case *unix.SockaddrUnix:
		//	fmt.Printf("Unix Socket Path: %s\n", addr.Name)

		default:
		}
	}
	return "", errors.New("unknown address type")
}
