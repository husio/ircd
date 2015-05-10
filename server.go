package ircd

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	VERSION = "0.1-awesome"
)

type Server struct {
	log     *log.Logger
	created time.Time

	mu       sync.RWMutex
	channels map[string]*Channel
	clients  map[string]*Client
	nicks    map[string]*Client
}

func NewServer() *Server {
	return &Server{
		created:  time.Now(),
		log:      log.New(os.Stderr, "IRCD ", log.Ltime),
		channels: make(map[string]*Channel),
		clients:  make(map[string]*Client),
		nicks:    make(map[string]*Client),
	}
}

func (srv *Server) Run(address string) error {
	ln, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	for {
		client, err := ln.Accept()
		if err != nil {
			return err
		}
		go srv.handleClient(client)
	}
}

func (srv *Server) handleClient(c net.Conn) {
	id := fmt.Sprintf("c%d", nextID())
	client := &Client{
		c:        c,
		id:       id,
		server:   srv,
		channels: make(map[string]*Channel),
	}

	srv.mu.Lock()
	srv.clients[client.id] = client
	srv.mu.Unlock()

	rd := bufio.NewReader(c)
	for {
		line, err := readRNLine(rd)
		if err != nil {
			srv.log.Printf("client %q read error: %s", client.id, err)
			srv.log.Printf("disconnecting client %q", client.id)
			return
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		chunks := strings.Split(string(line), " ")
		command := chunks[0]
		handler, ok := commandHandlers[command]
		if !ok {
			client.Send("%d %s %s :Unknown command", ERR_UNKNOWNCOMMAND, client.Nick, command)
			continue
		}
		handler(srv, client, chunks[1:]...)
	}
}

// readRNLine is reading from reader line terminated with \r\n and returs read
// bytes up to \r\n. Line ending characters are cut off.
func readRNLine(rd *bufio.Reader) ([]byte, error) {
	line := make([]byte, 0)
	for !bytes.HasSuffix(line, []byte("\r\n")) {
		chunk, err := rd.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		line = append(line, chunk...)
	}
	return line[:len(line)-2], nil
}

type Client struct {
	id       string
	c        net.Conn
	server   *Server
	channels map[string]*Channel
	Nick     string
	Name     string
}

// Send writes to client given line. If given line does not end with \r\n,
// those bytes will be send extra.
func (cli *Client) Send(line string, args ...interface{}) error {
	cli.server.log.Printf(line, args...)
	if _, err := fmt.Fprintf(cli.c, line, args...); err != nil {
		return err
	}
	if !strings.HasSuffix(line, "\r\n") {
		_, err := fmt.Fprint(cli.c, "\r\n")
		return err
	}
	return nil
}

func (cli *Client) Close() {
	cli.c.Close()
}

type Channel struct {
	name    string
	clients map[string]*Client
}

func newChannel(name string) *Channel {
	return &Channel{
		name:    name,
		clients: make(map[string]*Client),
	}
}
