// 链接到上游服务器，负责客户端和真实服务器之间的消息转发
package main

import (
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"time"
)

const (
	MessageClassLocal      = 1 //本地消息，本地处理
	MessageClassUpstream   = 2 //发往上游，上游处理
	MessageClassDownstream = 3 //上游下发，本地处理
)

const (
	MessageTypeConnect    = 1 //连接建立
	MessageTypeDisconnect = 2 //连接断开
	MessageTypeData       = 3 //数据消息
)

type Message struct {
	MessageClass int    `json:"message_class"` //消息处理方
	MessageType  int    `json:"message_type"`  //消息类型
	UUID         string `json:"uuid"`          //UUID
	IPStr        string `json:"ip_str"`        //真实请求IP:Port
	Length       int    `json:"length"`
	Data         []byte `json:"data"`
}

// 链路链接状态
const (
	Connected    = iota + 1 //已连接状态
	Disconnect              //需要断开
	Disconnected            //已断开状态
)

var messageChannel = make(chan Message, 10000)

type ConnectionInfo struct { // ConnectionInfo 客户端链路链接信息
	IPStr     string   //真实请求目的IP
	Conn      net.Conn //链接信息
	Status    int      //链接状态
	Timestamp int64    //时间搓
}

var connections = make(map[string]ConnectionInfo)

var conn net.Conn
var status = Disconnected

func initClientTls() bool {
	LOGI("init downstream with tls")
	serverAddr := ConfigParam.Server

	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	tmpConn, err := tls.Dial("tcp", serverAddr, tlsConfig)
	if err != nil {
		LOGE("downstream--->upstream, ", serverAddr, " connect, fail, ", err)
		return false
	} else {
		LOGI("downstream--->upstream, ", serverAddr, " connect, success")
	}
	conn = tmpConn
	status = Connected
	return true
}

func initClient() bool {
	LOGI("init downstream without tls")
	serverAddr := ConfigParam.Server

	tmpConn, err := net.Dial("tcp", serverAddr) //连接到上游服务器
	if err != nil {
		LOGE("downstream--->upstream, ", serverAddr, " connect, fail, ", err)
		return false
	} else {
		LOGI("downstream--->upstream, ", serverAddr, " connect, success")
	}
	conn = tmpConn
	status = Connected
	return true
}

func closeClient() {
	err := conn.Close()
	if err != nil {
		LOGE("downstream fail to close, ", err)
	} else {
		LOGI("downstream closed")
	}
	conn = nil
	status = Disconnected
}

func startClient() {
	go handleEvents()
	for {
		for status == Disconnected {
			if initClient() == true {
				break
			} else {
				time.Sleep(5 * time.Second) //若连接失败，5秒后重试
			}
		}
		rcvFromUpstream()
	}
}

func handleEvents() {
	LOGI("downstream start to handle events")
	for {
		select {
		case message := <-messageChannel:
			switch message.MessageClass {
			case MessageClassLocal:
				handleEventLocal(message) //处理本地消息
			case MessageClassDownstream:
				handleEventDownstream(message) //处理上游下发消息
			default:
				LOGE("Unknown message class:", message.MessageClass)
			}
		}
	}
}

func handleEventLocal(msg Message) {
	switch msg.MessageType {
	case MessageTypeConnect: //新建连接，上报上游
		msg.MessageClass = MessageClassUpstream
		data, err := json.Marshal(msg)
		if err != nil {
			LOGE(msg.UUID, " marshaling message, fail, ", err)
			return
		}
		_, err = sndToUpstream(conn, data)
		if err != nil {
			LOGE(msg.UUID, " downstream--->upstream, write, event-connct, fail, ", err)
			return
		} else {
			LOGD(msg.UUID, " downstream--->upstream, write, event-connect, success")
		}

	case MessageTypeDisconnect: //与客户端之间的连接断开
		connClient := connections[msg.UUID]
		if connClient.Status == Connected { //主动断开，上报状态
			msg.MessageClass = MessageClassUpstream
			data, err := json.Marshal(msg)
			if err != nil {
				LOGE(msg.UUID, " marshaling message, fail, ", err)
				return
			}
			_, err = sndToUpstream(conn, data)
			if err != nil {
				LOGE(msg.UUID, " downstream--->upstream, write, event-disconnct, fail, ", err)
				return
			} else {
				LOGD(msg.UUID, " downstream--->upstream, write, event-disconnect, success")
			}
		} //else if connClient.Status == Disconnect 被动断开，无需上报

		connClient.Status = Disconnected
		delete(connections, msg.UUID)

	case MessageTypeData: //客户端消息转发到上游
		msg.MessageClass = MessageClassUpstream
		data, err := json.Marshal(msg)
		if err != nil {
			LOGE(msg.UUID, "fail to marshaling message ", err)
			return
		}
		_, err = sndToUpstream(conn, data)
		if err != nil {
			LOGE(msg.UUID, " downstream--->upstream, write, event-data, fail, ", err)
			return
		} else {
			LOGD(msg.UUID, " downstream--->upstream, write, event-data, success")
		}
	}
}

func handleEventDownstream(msg Message) {
	connection, exists := connections[msg.UUID]
	if !exists {
		LOGE(msg.UUID, " connection not found")
		return
	}

	if msg.MessageType == MessageTypeData { //收到真实服务器数据
		length, err := connection.Conn.Write(msg.Data) //数据转发给客户端
		if err != nil {
			LOGE(msg.UUID, "client<---downstream, write, fail, ", err)
			return
		} else {
			LOGD(msg.UUID, "client<---downstream, write, success, need: ", msg.Length, " snd: ", length)
		}
	} else if msg.MessageType == MessageTypeDisconnect { //与真是服务器断开链接
		if connection.Status == Connected {
			connection.Status = Disconnect //修改状态后续断开
		}
	}
}

func AddEventConnect(uuid string, ipStr string) {
	message := Message{
		MessageClass: MessageClassLocal,
		MessageType:  MessageTypeConnect,
		UUID:         uuid,
		IPStr:        ipStr,
		Length:       0,
		Data:         nil,
	}
	messageChannel <- message
}

func AddEventDisconnect(uuid string) {
	message := Message{
		MessageClass: MessageClassLocal,
		MessageType:  MessageTypeDisconnect,
		UUID:         uuid,
		IPStr:        "",
		Length:       0,
		Data:         nil,
	}
	messageChannel <- message
}

func AddEventMsg(uuid string, buf []byte, len int) {
	message := Message{
		MessageClass: MessageClassLocal,
		MessageType:  MessageTypeData,
		UUID:         uuid,
		IPStr:        "",
		Length:       len,
		Data:         buf[:len],
	}
	messageChannel <- message
}

// 发送消息到上游
func sndToUpstream(conn net.Conn, data []byte) (n int, err error) {
	lenData := len(data)
	lenBuf := []byte{byte(lenData >> 8), byte(lenData & 0xff)}
	_, err = conn.Write(lenBuf) //发送长度
	if err == nil {
		length, err := conn.Write(data) //发送数据
		if err != nil {
			LOGE("downstream--->upstream, write, fail, ", err)
			return 0, err
		} else {
			LOGD("downstream--->upstream, write data: ", length, " need: ", lenData)
			return length, nil
		}
	} else {
		return 0, err
	}
}

func rcvFromUpstream() { // 接受上游消息
	LOGI("downstream start to rcv data")
	for {
		lengthBuf := make([]byte, 2) //读取长度部分
		length, err := io.ReadFull(conn, lengthBuf)
		if err != nil {
			LOGE("downstream<---upstream, read length, fail, ", err)
			closeClient()
			return
		} else {
			LOGD("downstream<---upstream, read length, success, length: ", length)
		}

		length = int(lengthBuf[0])<<8 + int(lengthBuf[1])
		dataBuf := make([]byte, length)
		lenData, err := io.ReadFull(conn, dataBuf)
		if err != nil {
			LOGE("downstream<---upstream, read data, fail, ", err)
			closeClient()
			return
		} else {
			LOGD("downstream<---upstream, read data, success, need: ", length, ", read: ", lenData)
		}

		var msg Message
		err = json.Unmarshal(dataBuf, &msg)
		if err == nil {
			messageChannel <- msg
		} else {
			LOGE("downstream unmarshalling message, fail, ", err)
		}
	}
}
