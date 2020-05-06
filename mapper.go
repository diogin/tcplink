package main

import (
	"bytes"
	"io"
	"net"
	"time"
)

func serveMapper(secret string, listen string, target string) {
	clients := make(chan *net.TCPConn)
	secKey := []byte(defaultSecret)
	if len(secret) < 16 {
		copy(secKey, secret)
	} else {
		copy(secKey, secret[:16])
	}
	go func() {
		addr, err := net.ResolveTCPAddr("tcp", target)
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
	addr, err := net.ResolveTCPAddr("tcp", listen)
	must(err)
	door, err := net.ListenTCP("tcp", addr)
	must(err)
	for {
		if finder, err := door.AcceptTCP(); err == nil {
			go relayMapper(finder, clients, secKey)
		}
	}
}

func relayMapper(finder *net.TCPConn, clients chan *net.TCPConn, secKey []byte) {
	var (
		client *net.TCPConn
		linked = false
	)
	defer func() {
		if !linked {
			finder.Close()
			if client != nil {
				client.Close()
			}
		}
	}()
	// read auth
	var auth [16]byte
	if err := finder.SetReadDeadline(time.Now().Add(shortTimeout)); err != nil {
		return
	}
	if _, err := io.ReadFull(finder, auth[:]); err != nil || !bytes.Equal(auth[:], secKey) {
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
	ok := [1]byte{'0'}
	if err := finder.SetWriteDeadline(time.Now().Add(shortTimeout)); err != nil {
		return
	}
	if _, err := finder.Write(ok[:]); err != nil {
		return
	}

	linked = true

	// now relay
	var state closeState
	state.setFrom(finder)
	go tcpRelay(finder, client, &state)
	tcpRelay(client, finder, &state)
}
