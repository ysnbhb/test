package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

func readFromServer(conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 2048)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if err.Error() != "EOF" {
				fmt.Println("Error reading from server:", err)
			}
			return
		}
		if n > 0 {
			fmt.Print(string(buf[:n]))
		}
	}
}

func main() {
	serverAddr := "localhost:8080" // Replace with your server's address and port

	// Connect to the TCP server
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		fmt.Println("Error connecting to server:", err)
		os.Exit(1)
	}

	// Start reading from the server in a separate goroutine
	go readFromServer(conn)

	// Set up reader and prompt user for their name
	reader := bufio.NewReader(os.Stdin)
	//fmt.Print("Enter your name: ")
	name, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("Error reading name:", err)
		return
	}
	name = strings.TrimSpace(name)

	// Send the name to the server
	_, err = fmt.Fprintf(conn, "%s\n", name)
	if err != nil {
		fmt.Println("Error sending name:", err)
		return
	}

	fmt.Println("Connected to server. Type your messages below:")

	for {
		message, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading message:", err)
			break
		}
		message = strings.TrimSpace(message)
		if message == "" {
			continue
		}

		// Send the message to the server
		_, err = conn.Write([]byte(message + "\n"))
		if err != nil {
			fmt.Println("Error sending message:", err)
			break
		}
	}
}
