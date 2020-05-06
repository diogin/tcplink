package main

import (
	"net"
)

func serveInner(listen string, target string, secret string) {
	addr, err := net.ResolveTCPAddr("tcp", listen)
	must(err)
	door, err := net.ListenTCP("tcp", addr)
	must(err)
	for {
		if client, err := door.AcceptTCP(); err == nil {
			go relayInner(client, target, secret)
		}
	}
}

func relayInner(client *net.TCPConn, target string, secret string) {
	// dial
	conn, err := net.DialTimeout("tcp", target, shortTimeout)
	if err != nil {
		client.Close()
		return
	}
	outer := conn.(*net.TCPConn)

	// now relay
	var state closeState
	state.setFrom(client)
	go tcpEncryptRelay(client, outer, reqHead, &state, secret)
	tcpDecryptRelay(outer, client, &state, secret)
}
