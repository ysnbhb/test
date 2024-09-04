package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
)

type Message struct {
	username string
	payload  string
}

type Server struct {
	listenAddr string
	ln         net.Listener
	conns      map[net.Conn]string // Map of connections to user names
	mu         sync.Mutex
	quich      chan struct{}
	msg        chan Message
}

func NewServer(s string) *Server {
	return &Server{
		listenAddr: s,
		conns:      make(map[net.Conn]string),
		quich:      make(chan struct{}),
		msg:        make(chan Message, 100),
	}
}

func (s *Server) start() error {
	ln, err := net.Listen("tcp", s.listenAddr)
	if err != nil {
		return err
	}
	defer ln.Close()
	s.ln = ln
	go s.acceptLoop()
	<-s.quich
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
		fmt.Println("Connection closed by", s.conns[conn])
		conn.Close()

		s.mu.Lock()
		delete(s.conns, conn)
		s.mu.Unlock()
	}()

	reader := bufio.NewReader(conn)
	fmt.Fprint(conn, "Enter your name: ")
	name, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("Error reading name from connection:", err)
		return
	}
	name = strings.TrimSpace(name)

	s.mu.Lock()
	s.conns[conn] = name
	s.mu.Unlock()

	welcomeMessage := "Welcome, " + name + "!\n"
	conn.Write([]byte(welcomeMessage))
	fmt.Fprintln(conn, name+" connected from "+conn.RemoteAddr().String())

	buf := make([]byte, 2048)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if err.Error() != "EOF" {
				fmt.Println("Error reading from connection:", err)
			}
			break
		}
		if n == 0 {
			fmt.Println("Client closed the connection:", conn.RemoteAddr().String())
			break
		}
		msgContent := strings.TrimSpace(string(buf[:n]))
		if msgContent != "" {
			s.msg <- Message{
				username: name,
				payload:  msgContent,
			}
		}
	}
}

func (s *Server) write() {
	for msg := range s.msg {
		s.mu.Lock()
		for conn := range s.conns {
			_, err := conn.Write([]byte(fmt.Sprintf("%s: %s\n", msg.username, msg.payload)))
			if err != nil {
				fmt.Println("Error writing to connection:", err)
				conn.Close()
				delete(s.conns, conn)
			}
		}
		s.mu.Unlock()
	}
}

func main() {
	ser := NewServer(":8080")
	go ser.write()
	if err := ser.start(); err != nil {
		fmt.Println("Error starting server:", err)
	}
}
