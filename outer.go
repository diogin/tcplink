package main

import (
	"net"
)

func serveOuter(listen string, target string, secret string) {
	addr, err := net.ResolveTCPAddr("tcp", listen)
	must(err)
	door, err := net.ListenTCP("tcp", addr)
	must(err)
	for {
		if inner, err := door.AcceptTCP(); err == nil {
			go relayOuter(inner, target, secret)
		}
	}
}

func relayOuter(inner *net.TCPConn, target string, secret string) {
	// dial
	conn, err := net.DialTimeout("tcp", target, shortTimeout)
	if err != nil {
		inner.Close()
		return
	}
	server := conn.(*net.TCPConn)

	// now relay
	var state closeState
	state.setFrom(inner)
	go tcpDecryptRelay(inner, server, &state, secret)
	tcpEncryptRelay(server, inner, resHead, &state, secret)
}
