package main

import (
	"net"
	"time"
)

func serveAgent(listen string, secret string) {
	addr, err := net.ResolveTCPAddr("tcp", listen)
	must(err)
	door, err := net.ListenTCP("tcp", addr)
	must(err)
	for {
		if proxy, err := door.AcceptTCP(); err == nil {
			go relayAgent(proxy, secret)
		}
	}
}

func relayAgent(proxy *net.TCPConn, secret string) {
	var (
		server *net.TCPConn
		linked = false
	)
	defer func() {
		if !linked {
			proxy.Close()
			if server != nil {
				server.Close()
			}
		}
	}()
	// read unit link
	if err := proxy.SetDeadline(time.Now().Add(shortTimeout)); err != nil {
		return
	}
	unit := newUnit(secret)
	unit.forDecrypt()
	kind, addr, err := unit.readFrom(proxy)
	if err != nil || kind != unitKindLink || len(addr) == 0 {
		return
	}
	// link server
	conn, err := net.DialTimeout("tcp", string(addr), shortTimeout)
	if err != nil {
		return
	}
	server = conn.(*net.TCPConn)
	// write unit ok
	unit.forEncrypt(resHead)
	if err := unit.writeTo(proxy, unitKindOK, 0); err != nil {
		return
	}
	// free unit
	unit = nil

	linked = true

	// now relay
	var state closeState
	state.setFrom(proxy)
	go tcpDecryptRelay(proxy, server, &state, secret)
	tcpEncryptRelay(server, proxy, resHead, &state, secret)
}
