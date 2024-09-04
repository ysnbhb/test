package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

type Message struct {
	Username string
	Payload  string
}

type Server struct {
	listenAddr string
	ln         net.Listener
	conns      map[net.Conn]string // Map of connections to user names
	mu         sync.Mutex
	quit       chan struct{}
	msg        chan Message
}

func NewServer(addr string) *Server {
	return &Server{
		listenAddr: addr,
		conns:      make(map[net.Conn]string),
		quit:       make(chan struct{}),
		msg:        make(chan Message, 100),
	}
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.listenAddr)
	if err != nil {
		return err
	}
	defer ln.Close()
	s.ln = ln

	go s.acceptLoop()
	go s.broadcast()

	<-s.quit
	close(s.msg)
	return nil
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer func() {
		s.mu.Lock()
		name := s.conns[conn]
		delete(s.conns, conn)
		s.mu.Unlock()
		conn.Close()
		fmt.Printf("Connection closed by %s\n", name)
	}()

	reader := bufio.NewReader(conn)
	fmt.Fprint(conn, "Enter your name: ")
	name, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Error reading name from connection: %v\n", err)
		return
	}
	name = strings.TrimSpace(name)

	s.mu.Lock()
	s.conns[conn] = name
	s.mu.Unlock()

	welcomeMessage := fmt.Sprintf("Welcome, %s!\n", name)
	conn.Write([]byte(welcomeMessage))
	fmt.Fprintf(conn, "%s connected from %s\n", name, conn.RemoteAddr().String())

	buf := make([]byte, 2048)
	for {
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		prompt := fmt.Sprintf("[%s][%s]: ", timestamp, name)
		fmt.Fprint(conn, prompt)

		n, err := conn.Read(buf)
		if err != nil {
			if err.Error() != "EOF" {
				fmt.Printf("Error reading from connection: %v\n", err)
			}
			break
		}
		if n == 0 {
			fmt.Printf("Client closed the connection: %s\n", conn.RemoteAddr().String())
			break
		}
		msgContent := strings.TrimSpace(string(buf[:n]))
		if msgContent != "" {
			s.msg <- Message{
				Username: name,
				Payload:  msgContent,
			}
		}
	}
}

func (s *Server) broadcast() {
	for msg := range s.msg {
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		formattedMessage := fmt.Sprintf("\n[%s][%s]: %s\n", timestamp, msg.Username, msg.Payload)
		s.mu.Lock()
		for conn, username := range s.conns {
			if username != msg.Username {
				_, err := conn.Write([]byte(formattedMessage))
				if err != nil {
					fmt.Printf("Error writing to connection: %v\n", err)
					conn.Close()
					delete(s.conns, conn)
				}
				timestamp := time.Now().Format("2006-01-02 15:04:05")
				prompt := fmt.Sprintf("[%s][%s]: ", timestamp, username)
				fmt.Fprint(conn, prompt)
			}
		}
		s.mu.Unlock()
	}
}

func main() {
	server := NewServer(":8080")
	if err := server.Start(); err != nil {
		fmt.Println("Error starting server:", err)
	}
}
