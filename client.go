// 链接代理服务器
package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

const (
	MessageTypeLocal      = "local"
	MessageTypeUpstream   = "upstream"
	MessageTypeDownstream = "downstream"
)

const (
	MessageClassConnect   = "connect"
	MessageClasDisconnect = "disconnect"
	MessageClasData       = "data"
)

type Message struct {
	MessageType  string `json:"message_type"`
	MessageClass string `json:"message_class"`
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
	// Define the server address and port
	serverAddr := ConfigParam.Server

	// Create a TLS configuration
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true, // Note: InsecureSkipVerify should be used only for testing purposes
	}

	// Establish a TLS connection to the server
	tmpConn, err := tls.Dial("tcp", serverAddr, tlsConfig)
	if err != nil {
		LOGE("failed to connect, ", err)
		return false
	}
	conn = tmpConn
	return true
}

func initClient() bool {
	LOGI("init client")
	// Define the server address and port
	serverAddr := ConfigParam.Server

	// Establish a connection to the server
	tmpConn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		LOGE("failed to connect, ", err)
		return false
	} else {
		LOGI("connected to server")
	}
	conn = tmpConn
	return true
}

func closeClient() {
	err := conn.Close()
	if err != nil {
		LOGE("failed to close client: %v", err)
	} else {
		LOGI("client closed")
	}
}

func startClient() {
	LOGI("client started")
	go handleEvents()

	buf := make([]byte, 10240)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			fmt.Println("Error reading:", err)
			return
		} else {
			fmt.Println("Reading from upstream：", n)
		}

		var msg Message
		err = json.Unmarshal(buf[:n], &msg)
		if err != nil {
			fmt.Println("Error unmarshaling message:", err)
			return
		}
		messageChannel <- msg
	}
}

func handleEventLocal(msg Message) {
	switch msg.MessageClass {
	case MessageClassConnect:
		msg.MessageType = MessageTypeUpstream
		data, err := json.Marshal(msg)
		if err != nil {
			LOGE("Error marshaling message:", err)
			return
		}
		_, err = conn.Write(data)
		if err != nil {
			LOGE("Error writing:", err)
			return
		} else {
			LOGD("send to event-connect to upstream, length:", len(data))
		}

	case MessageClasDisconnect:
		delete(connections, msg.UUID)
	case MessageClasData:
		msg.MessageType = MessageTypeUpstream
		data, err := json.Marshal(msg)
		if err != nil {
			LOGE("Error marshaling message:", err)
			return
		}
		_, err = conn.Write(data)
		if err != nil {
			LOGE("Error writing:", err)
			return
		} else {
			LOGE("send to event-data to upstream : ", len(data))
		}
	}
}

func handleEventDownstream(msg Message) {
	connection, exists := connections[msg.UUID]
	if !exists {
		LOGE("connection not found, uuid:", msg.UUID)
		return
	}

	if msg.MessageType == MessageClasData {
		_, err := connection.Conn.Write(msg.Data)
		if err != nil {
			LOGE("Error writing to client:", err, " uuid:", msg.UUID)
			return
		} else {
			LOGI("send to client : ", len(msg.Data), " uuid:", msg.UUID)
		}
	}
}

func handleEvents() {
	for {
		select {
		case message := <-messageChannel:
			switch message.MessageType {
			case MessageTypeLocal:
				handleEventLocal(message)
			case MessageTypeDownstream:
				handleEventDownstream(message)
			default:
			}
		}
	}
}

func clientAddEventConnect(uuid string, ipStr string, conn net.Conn) {
	connections[uuid] = ConnectionInfo{
		IPStr:     ipStr,
		Conn:      conn,
		Timestamp: time.Now().Unix(),
		Status:    Connected,
	}

	message := Message{
		MessageType:  MessageTypeLocal,
		MessageClass: MessageClassConnect,
		UUID:         uuid,
		IPStr:        ipStr,
		Length:       0,
		Data:         nil,
	}
	messageChannel <- message
}

func clientAddEventDisconnect(uuid string) {
	message := Message{
		MessageType:  MessageTypeLocal,
		MessageClass: MessageClasDisconnect,
		UUID:         uuid,
		IPStr:        "",
		Length:       0,
		Data:         nil,
	}
	messageChannel <- message
}

func clientAddEventMsg(uuid string, buf []byte, len int) {
	message := Message{
		MessageType:  MessageTypeLocal,
		MessageClass: MessageClasData,
		UUID:         uuid,
		IPStr:        "",
		Length:       len,
		Data:         buf[:len],
	}
	connection, exists := connections[uuid]
	if exists {
		connection.Timestamp = time.Now().Unix()
	}
	messageChannel <- message
}
