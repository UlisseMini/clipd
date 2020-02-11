package main

import (
	"encoding/gob"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/atotto/clipboard"
)

const maxClipSize = 100000 // 100kb

// Our current clipboard, for reference
var currentCB string

// mutex protecting clipboard
var mu sync.Mutex

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

		func() {
			mu.Lock()
			defer mu.Unlock()
			currentCB = buf
		}()
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

	serverConn, err := net.Dial("tcp", "localhost:1337")
	if err != nil {
		log.Fatal(err)
	}
	defer func() { must(serverConn.Close()) }()
	log.Printf("Connected to %s", serverConn.RemoteAddr())

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

		if cb != currentCB {
			log.Printf("Sending new clipboard %q", cb)
			err := enc.Encode(&cb)
			if err != nil {
				// TODO: retry if temporary
				log.Fatal(err)
			}

			func() {
				mu.Lock()
				defer mu.Unlock()
				currentCB = cb
			}()
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
