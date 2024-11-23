// 链接代理服务器
package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

type Message struct {
	MessageType string `json:"message_type"`
	UUID        string `json:"uuid"`
	IPStr       string `json:"ip_str"`
	Length      int    `json:"length"`
	Data        []byte `json:"data"`
}

var messageChannel = make(chan Message, 10000)

type ConnectionInfo struct {
	IPStr     string
	Conn      net.Conn
	Timestamp int64
}

var connections = make(map[string]ConnectionInfo)

var conn net.Conn

func initClient() bool {
	// Define the server address and port
	serverAddr := ConfigParam.Server

	// Create a TLS configuration
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true, // Note: InsecureSkipVerify should be used only for testing purposes
	}

	// Establish a TLS connection to the server
	conn, err := tls.Dial("tcp", serverAddr, tlsConfig)
	if err != nil {
		fmt.Println("failed to connect: %v", err)
		return false
	}
	defer conn.Close()
	// Send a message to the server
	message := "Hello, Server!"
	_, err = conn.Write([]byte(message))
	if err != nil {
		fmt.Println("failed to send message: %v", err)
	}
	fmt.Println("Sent message:", message)

	// Receive a response from the server
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Println("failed to read response: %v", err)
	}
	fmt.Println("Received response:", string(buf[:n]))
	fmt.Println("Connected to server:", serverAddr)

	// You can now use conn to communicate with the server
	return true
}

func closeClient() {
	err := conn.Close()
	if err != nil {
		fmt.Println("failed to close client: %v", err)
	} else {
		fmt.Println("client closed")
	}
}

func startClient() {
	go handleEvents()

	buf := make([]byte, 2048)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			fmt.Println("Error reading:", err)
			return
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

func handleEvents() {
	for {
		select {
		case message := <-messageChannel:
			switch message.MessageType {
			case "upstream":
				handleEventMsg(message)
			case "downstream":
				handleEventMsg(message)
			case "connect":
				handleEventConnection(message)
			case "disconnect":
				handleEventDisconnect(message)
			}
		}
	}
}

func handleEventConnection(msg Message) {
	connections[msg.UUID] = ConnectionInfo{
		IPStr:     msg.IPStr,
		Conn:      conn,
		Timestamp: time.Now().Unix(),
	}
}

func handleEventDisconnect(msg Message) {
	delete(connections, msg.UUID)
}

func handleEventMsg(msg Message) {
	if msg.MessageType == "downstream" {
		connection, exists := connections[msg.UUID]
		if !exists {
			fmt.Println("Error: connection not found")
			return
		}
		_, err := connection.Conn.Write(msg.Data)
		if err != nil {
			fmt.Println("Error writing:", err)
			return
		}
	} else {
		data, err := json.Marshal(msg)
		if err != nil {
			fmt.Println("Error marshaling message:", err)
			return
		}
		_, err = conn.Write(data)
		if err != nil {
			fmt.Println("Error writing:", err)
			return
		}
	}
}

func clientAddEventConnect(uuid string, ipStr string, conn net.Conn) {
	message := Message{
		MessageType: "connect",
		UUID:        uuid,
		IPStr:       ipStr,
		Length:      0,
		Data:        nil,
	}
	messageChannel <- message
}

func clientAddEventDisconnect(uuid string) {
	message := Message{
		MessageType: "disconnect",
		UUID:        uuid,
		IPStr:       "",
		Length:      0,
		Data:        nil,
	}
	messageChannel <- message
}

func clientAddEventMsg(uuid string, buf []byte, len int) {
	message := Message{
		MessageType: "upstream",
		UUID:        uuid,
		IPStr:       "",
		Length:      len,
		Data:        buf,
	}
	connection, exists := connections[uuid]
	if exists {
		connection.Timestamp = time.Now().Unix()
	}
	messageChannel <- message
}
