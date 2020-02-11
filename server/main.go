package main

import (
	"log"
	"net"
	"sync"
)

type client interface {
	update(clipboard string) error // update the clipboard
	subscribe() <-chan string      // new clipboards from the client
	String() string                // also require stringer interface
}

type server struct {
	// DO NOT MODIFY; use setClipboard
	clipboard string

	// for changing the clipboard, on reads this is not locked
	mu *sync.Mutex

	// when a new clipboard arives from a client.
	// clients messages are fanned into this.
	newCB chan string

	clients []client
}

// setClipboard sets the clipboard of the server, and notifies all clients.
// it is threadsafe.
func (s *server) setClipboard(cb string) {
	log.Printf("setting clipboard to %q, aquiring lock...", cb)
	s.mu.Lock()
	defer s.mu.Unlock()
	log.Printf("lock aquired")
	s.clipboard = cb

	// TODO: use goroutines to make this process faster, each client update()
	// uses the network.
	for i := 0; i < len(s.clients); i++ {
		client := s.clients[i]
		err := client.update(cb)
		if err != nil {
			// remove this client from the list, since they caused an error
			s.clients = append(s.clients[:i], s.clients[i+1:]...)
			log.Printf("Removed client %s because of %v", client, err)
		}
	}
}

// add a client to the server, calling client.update and client.subscribe
func (s *server) addClient(client client) {
	s.clients = append(s.clients, client)

	// Fan in all the clients
	go func(c <-chan string) {
		for cb := range c {
			log.Printf("Got %q sending on newCB...", cb)
			s.newCB <- cb
		}
	}(client.subscribe())

	client.update(s.clipboard) // TODO: Lock
}

func (s *server) step() error {
	cb := <-s.newCB
	log.Printf("[server] Recieved new clipboard %q", cb)
	s.setClipboard(cb)
	return nil
}

// run the server until an error occures
func (s *server) run() error {
	for {
		s.step()
	}
}

func newServer() *server {
	return &server{
		newCB: make(chan string),
		mu:    &sync.Mutex{},
	}
}

// tcpClient works over tcp
type tcpClient struct {
	conn      net.Conn
	out       chan string
	bufsize   int
	clipboard string
}

func (t *tcpClient) update(cb string) error {
	if t.clipboard == cb {
		return nil
	}

	_, err := t.conn.Write([]byte(cb))
	log.Printf("Wrote new clipboard %q to %s", cb, t.conn.RemoteAddr())
	return err
}

func (t *tcpClient) subscribe() <-chan string {
	return t.out
}

func (t *tcpClient) String() string {
	return t.conn.RemoteAddr().String()
}

func (t *tcpClient) run() error {
	buf := make([]byte, t.bufsize)
	for {
		n, err := t.conn.Read(buf)
		if err != nil {
			return err
		}
		log.Printf("Read %d bytes from %s", n, t.conn.RemoteAddr())

		t.clipboard = string(buf[:n])
		t.out <- string(buf[:n])
	}
}

func handle(s *server, conn net.Conn) {
	log.Printf("Handling %s", conn.RemoteAddr())
	c := &tcpClient{
		conn:    conn,
		out:     make(chan string),
		bufsize: 1024,
	}
	s.addClient(c)

	if err := c.run(); err != nil {
		log.Print(err)
	}
}

func main() {
	l, err := net.Listen("tcp", ":1337")
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	s := newServer()
	go func() {
		log.Panic(s.run())
	}()

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Print(err)
			return
		}
		go handle(s, conn)
	}
}
