package main

import (
	"encoding/gob"
	"flag"
	"io"
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
	log.Printf("setting clipboard to %q", cb)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clipboard = cb

	// for clients that need to be deleted
	deleteIndexes := make(chan int)

	wg := &sync.WaitGroup{}
	wg.Add(len(s.clients))
	for i := 0; i < len(s.clients); i++ {
		client := s.clients[i]
		go func(i int) {
			defer wg.Done()
			err := client.update(cb)
			if err != nil {
				// remove this client from the list, since they caused an error
				log.Printf("Removing client %s because of %v", client, err)
				deleteIndexes <- i
			}
		}(i)
	}

	go func() {
		wg.Wait()
		close(deleteIndexes)
	}()

	for i := range deleteIndexes {

		s.clients[i] = s.clients[len(s.clients)-1] // set it to the last one
		s.clients = s.clients[:len(s.clients)-1]   // chop off the last one

		log.Printf("Removed client %s", s.clients[i])
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

// tcpClient works over tcp using gob (encoding/gob)
type tcpClient struct {
	conn net.Conn
	enc  *gob.Encoder
	dec  *gob.Decoder

	out       chan string
	bufsize   int
	clipboard string
}

func (t *tcpClient) update(cb string) error {
	if t.clipboard == cb {
		return nil
	}

	err := t.enc.Encode(cb)
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
	var buf string
	for {
		dec := gob.NewDecoder(io.LimitReader(t.conn, int64(t.bufsize)))
		err := dec.Decode(&buf)
		if err != nil {
			return err
		}

		t.clipboard = buf
		t.out <- buf
	}
}

func handle(s *server, conn net.Conn) {
	log.Printf("Handling %s", conn.RemoteAddr())
	c := &tcpClient{
		conn:    conn,
		enc:     gob.NewEncoder(conn),
		out:     make(chan string),
		bufsize: 1024,
	}
	s.addClient(c)

	if err := c.run(); err != nil {
		log.Print(err)
	}
}

func main() {
	port := new(string)
	flag.StringVar(port, "p", "1337", "port for the server to listen on")
	flag.Parse()

	l, err := net.Listen("tcp", ":"+*port)
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	s := newServer()
	go func() {
		log.Panic(s.run())
	}()

	log.Printf("Listening on port %s...", *port)
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Print(err)
			return
		}
		go handle(s, conn)
	}
}
