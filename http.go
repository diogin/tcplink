package main

import (
	"bufio"
	"bytes"
	"net"
	"net/url"
	"strings"
	"time"
)

func serveHttp(listen string) {
	addr, err := net.ResolveTCPAddr("tcp", listen)
	must(err)
	door, err := net.ListenTCP("tcp", addr)
	must(err)
	for {
		if client, err := door.AcceptTCP(); err == nil {
			go relayHttp(client)
		}
	}
}

func relayHttp(client *net.TCPConn) {
	var (
		server *net.TCPConn
		linked = false
	)
	defer func() {
		if !linked {
			client.Close()
			if server != nil {
				server.Close()
			}
		}
	}()

	var (
		buffer []byte
		first  []byte
		https  bool
		addr   = ""
	)
	// ---> request
	if err := client.SetDeadline(time.Now().Add(shortTimeout)); err != nil {
		return
	}
	reader := bufio.NewReaderSize(client, 4096)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return
		}
		if first == nil {
			first = line
		}
		buffer = append(buffer, line...)
		if len(line) <= 2 {
			break
		}
	}
	if n := reader.Buffered(); n > 0 {
		left, err := reader.Peek(n)
		if err != nil {
			return
		}
		buffer = append(buffer, left...)
	}
	parts := bytes.SplitN(first, httpSpace, 3)
	if len(parts) != 3 || bytes.Index(parts[2], httpVer) != 0 {
		return
	}
	if bytes.Equal(parts[0], httpConnect) {
		https = true
		addr = string(parts[1])
	} else {
		https = false
		u, err := url.Parse(string(parts[1]))
		if err != nil {
			return
		}
		addr = u.Host
		if strings.IndexByte(addr, ':') == -1 {
			addr += ":80"
		}
	}
	// dial server
	conn, err := net.DialTimeout("tcp", addr, shortTimeout)
	if err != nil {
		return
	}
	server = conn.(*net.TCPConn)
	if https {
		// <--- response
		if _, err := client.Write(http200); err != nil {
			return
		}
	} else {
		// write the data read
		if _, err := server.Write(buffer); err != nil {
			return
		}
	}

	linked = true

	// now relay
	var state closeState
	state.setFrom(client)
	go tcpRelay(client, server, &state)
	tcpRelay(server, client, &state)
}

var (
	httpSpace   = []byte(" ")
	httpConnect = []byte("CONNECT")
	httpVer     = []byte("HTTP/1.")
	http200     = []byte("HTTP/1.1 200 Connection Established\r\n\r\n")
)
