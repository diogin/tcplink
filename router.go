package main

import (
	"bytes"
	"net"
	"time"
)

func serveRouter(args map[string]string) {
	clients := make(chan *net.TCPConn)
	go func() {
		addr, err := net.ResolveTCPAddr("tcp", args["target"])
		must(err)
		gate, err := net.ListenTCP("tcp", addr)
		must(err)
		defer gate.Close()
		for {
			if client, err := gate.AcceptTCP(); err == nil {
				select {
				case clients <- client:
				default:
					client.Close()
				}
			}
		}
	}()
	addr, err := net.ResolveTCPAddr("tcp", args["listen"])
	must(err)
	door, err := net.ListenTCP("tcp", addr)
	must(err)
	secret := args["secret"]
	for {
		if broker, err := door.AcceptTCP(); err == nil {
			go relayRouter(broker, clients, secret)
		}
	}
}

func relayRouter(broker *net.TCPConn, clients chan *net.TCPConn, secret string) {
	var (
		client *net.TCPConn
		linked = false
	)
	defer func() {
		if !linked {
			broker.Close()
			if client != nil {
				client.Close()
			}
		}
	}()
	// read auth
	if err := broker.SetReadDeadline(time.Now().Add(shortTimeout)); err != nil {
		return
	}
	unit := newUnit(secret)
	unit.forDecrypt()
	kind, auth, err := unit.readFrom(broker)
	if err != nil {
		return
	}
	if kind != unitKindAuth || !bytes.Equal(auth, unit.secKey) {
		return
	}
	// wait client
	timer := time.NewTimer(shortTimeout / 2)
	select {
	case <-timer.C:
		timer.Stop()
		return
	case client = <-clients:
		timer.Stop()
	}
	// write ok
	if err := broker.SetWriteDeadline(time.Now().Add(shortTimeout)); err != nil {
		return
	}
	unit.forEncrypt(resHead)
	if err := unit.writeTo(broker, unitKindOK, 0); err != nil {
		return
	}
	// free unit
	unit = nil

	linked = true

	// now relay
	var state closeState
	state.setFrom(broker)
	go tcpDecryptRelay(broker, client, &state, secret)
	tcpEncryptRelay(client, broker, resHead, &state, secret)
}
