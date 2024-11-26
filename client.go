package main

import (
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"io"
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
		LOGE("client connect to server ", serverAddr, " failed, ", err)
		return false
	} else {
		LOGI("client connect to server ", serverAddr, " success")
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
		LOGE("client connect to server ", serverAddr, " failed, ", err)
		return false
	} else {
		LOGI("client connect to server ", serverAddr, " success")
	}
	conn = tmpConn
	status = Connected
	return true
}

func closeClient() {
	err := conn.Close()
	if err != nil {
		LOGE("client fail to close, ", err)
	} else {
		LOGI("client closed")
	}
}

// Todo: when client is closed, the connection should be restarted
func startClient() {
	LOGI("client started")
	go handleEvents()

	for {
		lengthBuf := make([]byte, 4)
		lenData, err := io.ReadFull(conn, lengthBuf)
		if err != nil {
			if err != io.EOF {
				LOGE("downstream<---upstream, ", err)
			} else {
				LOGI("downstream<---upstream, ", err)
				return
			}
		} else {
			LOGI("downstream<---upstream, read length, ", lenData)
		}

		length := binary.BigEndian.Uint32(lengthBuf)
		dataBuf := make([]byte, length)
		len, err := io.ReadFull(conn, dataBuf)
		if err != nil {
			LOGE("downstream<---upstream, read data, failed, ", err)
			return
		} else {
			LOGI("downstream<---upstream, read data, need: ", length, ", read: ", len, " total: ", len+4)
		}

		var msg Message
		err = json.Unmarshal(dataBuf, &msg)
		if err != nil {
			LOGE("downstream fail to unmarshaling message,", err)
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
		LOGI(msg)
		data, err := json.Marshal(msg)
		if err != nil {
			LOGE(msg.UUID, " fail to marshaling message, ", err)
			return
		}
		length, err := sndToUpstream(conn, data)
		if err != nil {
			LOGE(msg.UUID, " downstream--->upstream, event-connct, ", err)
			return
		} else {
			LOGD(msg.UUID, " downstream--->upstream, event-connect, snd: ", length)
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
		length, err := sndToUpstream(conn, data)
		if err != nil {
			LOGE(msg.UUID, "downstream--->upstream, event-data, fail, ", err)
			return
		} else {
			LOGE(msg.UUID, " downstream--->upstream, event-data, send: ", length)
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
		length, err := connection.Conn.Write(msg.Data)
		if err != nil {
			LOGE(msg.UUID, "client<---downstream, wtire, fail, ", err)
			return
		} else {
			LOGI(msg.UUID, "client<---downstream, write, need: ", msg.Length, " snd: ", length)
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

func sndToUpstream(conn net.Conn, data []byte) (n int, err error) {
	length := uint32(len(data))

	buf := make([]byte, 4+length)
	binary.BigEndian.PutUint32(buf[:4], length)
	copy(buf[4:], data)

	return conn.Write(buf)
}
