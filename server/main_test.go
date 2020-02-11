package main

import (
	"io/ioutil"
	"log"
	"testing"
)

type testClient struct {
	updates   chan string
	clipboard string
}

func (t *testClient) update(clipboard string) error {
	t.clipboard = clipboard
	return nil
}

func (t *testClient) subscribe() <-chan string {
	return t.updates
}

func (t *testClient) String() string {
	return "[tc] " + t.clipboard
}

func newTestClient(updates chan string) *testClient {
	return &testClient{
		updates: updates,
	}
}

func TestServer(t *testing.T) {
	if !testing.Verbose() {
		log.SetOutput(ioutil.Discard)
	}

	server := newServer()

	ch1 := make(chan string)
	c1 := newTestClient(ch1)
	server.addClient(c1)

	ch2 := make(chan string)
	c2 := newTestClient(ch2)
	server.addClient(c2)

	// Run the test
	ch1 <- "foo"
	if err := server.step(); err != nil {
		t.Fatal(err)
	}
	if c1.clipboard != "foo" {
		t.Fatalf("want \"foo\" got %q", c1.clipboard)
	}

	if c2.clipboard != "foo" {
		t.Fatalf("want \"foo\" got %q", c2.clipboard)
	}
}
