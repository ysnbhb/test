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
	User       map[string]bool
}

var MaxUser int = 10

func NewServer(addr string) *Server {
	return &Server{
		listenAddr: addr,
		conns:      make(map[net.Conn]string),
		quit:       make(chan struct{}),
		msg:        make(chan Message, MaxUser),
		User:       make(map[string]bool),
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

	<-s.quit
	close(s.msg)
	return nil
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.ln.Accept()
		if len(s.conns) >= MaxUser {
			fmt.Fprintln(conn, "sorry bro but chat had max user \n")
			return
		}
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}
		go s.handleConnection(conn)
		go s.broadcast()
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer func() {
		s.mu.Lock()
		name := s.conns[conn]
		delete(s.conns, conn)
		delete(s.User, name)
		s.mu.Unlock()
		conn.Close()

		// Broadcast leave message
		leaveChat := fmt.Sprintf("%s -> has left our chat...\n", name)
		s.msg <- Message{
			Username: "Server",
			Payload:  leaveChat,
		}
	}()
	s.User["Server"] = true

	reader := bufio.NewReader(conn)
	name := ""
	for {
		fmt.Fprint(conn, "Enter your name: ")
		nameInput, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Error reading name from connection: %v\n", err)
			return
		}
		name = strings.TrimSpace(nameInput)
		if name == "" {
			continue
		}
		s.mu.Lock()
		if _, exists := s.User[name]; !exists {
			s.conns[conn] = name
			s.User[name] = true
			s.mu.Unlock()
			break
		}
		s.mu.Unlock()
		fmt.Fprint(conn, "This name is already in use. Please choose another name.\n")
	}
	joinMessage := fmt.Sprintf("\n%s -> has joined our chat...\n", name)
	for conn, username := range s.conns {
		if username != name {
			conn.Write([]byte(joinMessage))
			timestamp := time.Now().Format("2006-01-02 15:04:05")
			prompt := fmt.Sprintf("[%s][%s]: ", timestamp, username)
			fmt.Fprint(conn, prompt)
		}
	}

	// Broadcast join message
	// s.msg <- Message{
	// 	Username: "Server",
	// 	Payload:  joinMessage,
	// }

	//s.broadcast()

	// Handle prompt and user messages
	buf := make([]byte, 2048)
	for {
		// Send prompt after join message
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
			if msg.Username == "Server" {
				_, err := conn.Write([]byte("\n" + msg.Payload))
				if err != nil {
					fmt.Printf("Error writing to connection: %v\n", err)
					conn.Close()
					delete(s.conns, conn)
				}
				// Ensure prompt is correctly formatted
				timestamp := time.Now().Format("2006-01-02 15:04:05")
				prompt := fmt.Sprintf("[%s][%s]: ", timestamp, username)
				fmt.Fprint(conn, prompt)
			} else if username != msg.Username {
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
