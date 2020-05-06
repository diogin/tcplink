package main

import (
	"net"
)

func serveRelay(listen string, target string) {
	addr, err := net.ResolveTCPAddr("tcp", listen)
	must(err)
	door, err := net.ListenTCP("tcp", addr)
	must(err)
	for {
		if client, err := door.AcceptTCP(); err == nil {
			go relay(client, target)
		}
	}
}

func relay(client *net.TCPConn, target string) {
	// dial
	conn, err := net.DialTimeout("tcp", target, shortTimeout)
	if err != nil {
		client.Close()
		return
	}
	server := conn.(*net.TCPConn)

	// now relay
	var state closeState
	state.setFrom(client)
	go tcpRelay(client, server, &state)
	tcpRelay(server, client, &state)
}
