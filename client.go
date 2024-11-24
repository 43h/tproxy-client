// 链接代理服务器
package main

import (
	"crypto/tls"
	"encoding/json"
	"net"
	"time"
)

const (
	MessageClassLocal      = "local"
	MessageClassUpstream   = "upstream"
	MessageClassDownstream = "downstream"
)

const (
	MessageTypeConnect    = "connect"
	MessageTypeDisconnect = "disconnect"
	MessageTypeData       = "data"
)

type Message struct {
	MessageClass string `json:"message_class"`
	MessageType  string `json:"message_type"`
	UUID         string `json:"uuid"`
	IPStr        string `json:"ip_str"`
	Length       int    `json:"length"`
	Data         []byte `json:"data"`
}

const (
	Connected = iota + 1
	Disconnected
)

var messageChannel = make(chan Message, 10000)

type ConnectionInfo struct {
	IPStr     string
	Conn      net.Conn
	Status    int
	Timestamp int64
}

var connections = make(map[string]ConnectionInfo)

var conn net.Conn
var status int

func initClientTls() bool {
	LOGI("init client with tls")
	serverAddr := ConfigParam.Server

	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	tmpConn, err := tls.Dial("tcp", serverAddr, tlsConfig)
	if err != nil {
		LOGE("fail to connect to ", serverAddr, " ", err)
		return false
	} else {
		LOGI("connected to server ", serverAddr)
	}
	conn = tmpConn
	status = Connected
	return true
}

func initClient() bool {
	LOGI("init client without tls")
	serverAddr := ConfigParam.Server

	tmpConn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		LOGE("failed to connect, ", err)
		return false
	} else {
		LOGI("connected to server ", serverAddr)
	}
	conn = tmpConn
	return true
}

func closeClient() {
	err := conn.Close()
	if err != nil {
		LOGE("fail to close client: %v", err)
	} else {
		LOGI("client closed")
	}
}

// Todo: when client is closed, the connection should be restarted
func startClient() {
	LOGI("client started")
	go handleEvents()

	buf := make([]byte, 10240)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			LOGE("fail to read from upstream ", err)
			return
		} else {
			LOGI("Reading from upstream, length ", n)
		}

		var msg Message
		err = json.Unmarshal(buf[:n], &msg)
		if err != nil {
			LOGE("fail to unmarshaling message,", err)
			return
		}
		messageChannel <- msg
	}
}

func handleEvents() {
	for {
		select {
		case message := <-messageChannel:
			switch message.MessageClass {
			case MessageClassLocal:
				handleEventLocal(message)
			case MessageClassDownstream:
				handleEventDownstream(message)
			default:
				LOGE("Unknown message class:", message.MessageClass)
			}
		}
	}
}

func handleEventLocal(msg Message) {
	switch msg.MessageType {
	case MessageTypeConnect:
		msg.MessageClass = MessageClassUpstream
		data, err := json.Marshal(msg)
		if err != nil {
			LOGE(msg.UUID, "fail to marshaling message, ", err)
			return
		}
		_, err = conn.Write(data)
		if err != nil {
			LOGE(msg.UUID, " fail to send event-connct to upstream, ", err)
			return
		} else {
			LOGD(msg.UUID, " send to event-connect to upstream, length:", len(data))
		}

	case MessageTypeDisconnect:
		delete(connections, msg.UUID)
		//Todo: send disconnect message to upstream to notify the disconnection
	case MessageTypeData:
		msg.MessageClass = MessageClassUpstream
		data, err := json.Marshal(msg)
		if err != nil {
			LOGE(msg.UUID, "fail to marshaling message ", err)
			return
		}
		_, err = conn.Write(data)
		if err != nil {
			LOGE(msg.UUID, " fail to send event-data to upstream ", err)
			return
		} else {
			LOGE(msg.UUID, " send event-data to upstream, length ", len(data))
		}
	}
}

func handleEventDownstream(msg Message) {
	connection, exists := connections[msg.UUID]
	if !exists {
		LOGE(msg.UUID, " connection not found")
		return
	}

	if msg.MessageType == MessageTypeData {
		_, err := connection.Conn.Write(msg.Data)
		if err != nil {
			LOGE(msg.UUID, " fail to writing to client, ", err)
			return
		} else {
			LOGI(msg.UUID, " send to client, length: ", len(msg.Data))
		}
	}
}

func AddEventConnect(uuid string, ipStr string, conn net.Conn) {
	connections[uuid] = ConnectionInfo{
		IPStr:     ipStr,
		Conn:      conn,
		Timestamp: time.Now().Unix(),
		Status:    Connected,
	}

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
