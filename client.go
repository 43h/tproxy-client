//链接代理服务器
package main

import (
	"crypto/tls"
	"log"
)

func Client() {
	// Define the server address and port
	serverAddr := "server.example.com:443"

	// Create a TLS configuration
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true, // Note: InsecureSkipVerify should be used only for testing purposes
	}

	// Establish a TLS connection to the server
	conn, err := tls.Dial("tcp", serverAddr, tlsConfig)
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()
	// Send a message to the server
	message := "Hello, Server!"
	_, err = conn.Write([]byte(message))
	if err != nil {
		log.Fatalf("failed to send message: %v", err)
	}
	log.Println("Sent message:", message)

	// Receive a response from the server
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		log.Fatalf("failed to read response: %v", err)
	}
	log.Println("Received response:", string(buf[:n]))
	log.Println("Connected to server:", serverAddr)

	// You can now use conn to communicate with the server
}