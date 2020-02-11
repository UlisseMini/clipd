package main

import (
	"encoding/gob"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/atotto/clipboard"
)

const maxClipSize = 100000 // 100kb

// reader is ran in a goroutine to read new clipboard states from the server.
func reader(srv net.Conn) {
	var buf string

	for {
		time.Sleep(time.Second)

		dec := gob.NewDecoder(srv)
		err := dec.Decode(&buf)
		if err != nil {
			if isTemp(err) {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				continue
			} else {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				// TODO: try to reconnect to server, loop until successful?
				break
			}
		}

		if err := clipboard.WriteAll(buf); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			break // TODO: Retry
		}
		log.Printf("Set clipboard to %q", buf)
	}
}

func main() {
	if clipboard.Unsupported {
		panic("clipboard not supported")
	}

	serverConn, err := net.Dial("tcp", "192.168.86.47:1337")
	if err != nil {
		log.Fatal(err)
	}
	defer func() { must(serverConn.Close()) }()
	log.Printf("Connected to %s", serverConn.RemoteAddr())

	var oldCB string
	go func() {
		reader(serverConn)
		log.Fatal("reader exited")
	}()

	enc := gob.NewEncoder(serverConn)
	for {
		time.Sleep(time.Second)

		cb, err := clipboard.ReadAll()
		if err != nil {
			continue
		}

		if cb != oldCB {
			log.Printf("Sending new clipboard %q", cb)
			err := enc.Encode(&cb)
			if err != nil {
				// TODO: retry if temporary
				log.Fatal(err)
			}

			oldCB = cb
		}
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

// casts to a net.Error then returns Temporary()
func isTemp(err error) bool {
	if nerr, ok := err.(net.Error); ok {
		return nerr.Temporary()
	}
	return false
}
