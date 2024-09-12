package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
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
	file       *os.File
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
	if len(s.listenAddr) == 1 {
		s.listenAddr = ln.Addr().String()
	}
	fmt.Printf("Listening on the port %s\n", s.listenAddr)
	namefile := fmt.Sprintf("chat%s.txt", s.listenAddr)
	os.Remove(namefile)
	file, err := os.Create(namefile)
	if err != nil {
		return err
	}
	defer func() {
		ln.Close()
		file.Close()
	}()
	s.ln = ln
	s.file = file

	go s.acceptLoop()

	<-s.quit
	os.Remove(namefile)
	close(s.msg)
	return nil
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.ln.Accept()
		if len(s.conns) >= MaxUser {
			fmt.Fprintln(conn, "sorry bro but chat had max user")
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
		leaveChat := fmt.Sprintf("\n%s has left our chat...\n", name)
		s.ServEMessage(leaveChat, name)
	}()
	file, _ := os.ReadFile("logo.txt")
	conn.Write(file)
	reader := bufio.NewReader(conn)
	name := ""
	for {
		fmt.Fprint(conn, "[ENTER YOUR NAME]:")
		nameInput, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Error reading name from connection: %v\n", err)
			return
		}
		name = HandelInput(nameInput)
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
	joinMessage := fmt.Sprintf("\n%s  has joined our chat...\n", name)
	s.ServEMessage(joinMessage, name)
	last, _ := os.ReadFile(fmt.Sprintf("chat%s.txt", s.listenAddr))
	PrintLastMessage(last, conn)
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
		msgContent := HandelInput(string(buf[:n]))
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
		formattedMessage := fmt.Sprintf("[%s][%s]: %s\n", timestamp, msg.Username, msg.Payload)
		s.mu.Lock()
		for conn, username := range s.conns {
			s.file.WriteString((formattedMessage))
			if username != msg.Username {
				_, err := conn.Write([]byte("\n" + formattedMessage))
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
	arg := os.Args[1:]
	port := ""
	if len(arg) == 0 {
		port = ":8989"
	} else if len(arg) == 1 {
		port = fmt.Sprintf(":%s", arg[0])
	} else {
		fmt.Println("[USAGE]: ./TCPChat $port")
	}
	server := NewServer(port)

	if err := server.Start(); err != nil {
		fmt.Println("Error starting server:", err)
	}
}

func HandelInput(s string) string {
	res := ""
	for _, t := range s {
		if t < 32 {
			continue
		} else {
			res += string(t)
		}
	}
	return strings.TrimSpace(res)
}

func (s *Server) ServEMessage(str string, name string) {
	for conn, username := range s.conns {
		if username != name {
			conn.Write([]byte(str))
			timestamp := time.Now().Format("2006-01-02 15:04:05")
			prompt := fmt.Sprintf("[%s][%s]: ", timestamp, username)
			fmt.Fprint(conn, prompt)
		}
	}
}

func PrintLastMessage(last []byte, conn net.Conn) {
	_, err := conn.Write(last)
	if err != nil {
		fmt.Println(err)
	}
}
