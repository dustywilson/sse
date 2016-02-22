package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

// Broker is ...
type Broker struct {
	Notifier       chan []byte
	newClients     chan chan []byte
	closingClients chan chan []byte
	clients        map[chan []byte]bool
}

// NewServer is ...
func NewServer() *Broker {
	b := &Broker{
		Notifier:       make(chan []byte, 1),
		newClients:     make(chan chan []byte),
		closingClients: make(chan chan []byte),
		clients:        make(map[chan []byte]bool),
	}
	go b.listen()
	return b
}

func (b *Broker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("ServeHTTP: %+v\n", r)

	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-type", "text/event-stream")
	w.Header().Set("Cache-control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-control-allow-origin", "*")

	messageChan := make(chan []byte)

	b.newClients <- messageChan

	defer func() {
		b.closingClients <- messageChan
	}()

	notify := w.(http.CloseNotifier).CloseNotify()

	go func() {
		<-notify
		b.closingClients <- messageChan
	}()

	for {
		fmt.Fprintf(w, "data: %s\n\n", <-messageChan)
		f.Flush()
	}
}

func (b *Broker) listen() {
	for {
		select {
		case s := <-b.newClients:
			b.clients[s] = true
			log.Printf("Client added.  %d clients registered", len(b.clients))
		case s := <-b.closingClients:
			delete(b.clients, s)
			log.Printf("Client removed.  %d clients registered", len(b.clients))
		case e := <-b.Notifier:
			for ch := range b.clients {
				ch <- e
			}
		}
	}
}

func main() {
	b := NewServer()

	go func() {
		for {
			time.Sleep(time.Second * 2)
			e := fmt.Sprintf("Current time: %v", time.Now())
			log.Println("EVENT")
			b.Notifier <- []byte(e)
		}
	}()

	http.Handle("/", http.FileServer(http.Dir("www")))
	http.Handle("/sse", b)

	log.Fatal("HTTP error: ", http.ListenAndServe(":7654", nil))
}
