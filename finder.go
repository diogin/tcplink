package main

import (
	"io"
	"net"
	"time"
)

func serveFinder(secret string, listen string, target string) {
	next := make(chan bool, 4)
	secKey := []byte(defaultSecret)
	if len(secret) < 16 {
		copy(secKey, secret)
	} else {
		copy(secKey, secret[:16])
	}
	for i := 0; i < 4; i++ {
		go relayFinder(next, secKey, listen, target)
	}
	for {
		<-next
		go relayFinder(next, secKey, listen, target)
	}
}

func relayFinder(next chan bool, secKey []byte, listen string, target string) {
	var (
		conn   net.Conn
		err    error
		mapper *net.TCPConn
		server *net.TCPConn
	)
start:
	if mapper != nil {
		mapper.Close()
	}
	// dial mapper
	conn, err = net.DialTimeout("tcp", target, shortTimeout)
	if err != nil {
		time.Sleep(time.Second)
		goto start
	}
	mapper = conn.(*net.TCPConn)
	// write auth
	if err = mapper.SetDeadline(time.Now().Add(shortTimeout)); err != nil {
		goto start
	}
	if _, err = mapper.Write(secKey); err != nil {
		goto start
	}
	// read ok
	var ok [1]byte
	if _, err = io.ReadFull(mapper, ok[:]); err != nil || ok[0] != '0' {
		goto start
	}
	next <- true
	// dial server
	conn, err = net.DialTimeout("tcp", listen, shortTimeout)
	if err != nil {
		mapper.Close()
		return
	}
	server = conn.(*net.TCPConn)

	// now relay
	var state closeState
	state.setFrom(server)
	go tcpRelay(server, mapper, &state)
	tcpRelay(mapper, server, &state)
}
