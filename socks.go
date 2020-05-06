package main

import (
	"io"
	"net"
	"strconv"
	"time"
)

func serveSocks(listen string, target string, secret string) {
	addr, err := net.ResolveTCPAddr("tcp", listen)
	must(err)
	door, err := net.ListenTCP("tcp", addr)
	must(err)
	for {
		if client, err := door.AcceptTCP(); err == nil {
			go relaySocks(client, target, secret)
		}
	}
}

func relaySocks(client *net.TCPConn, target string, secret string) {
	var (
		agent  *net.TCPConn
		linked = false
	)
	defer func() {
		if !linked {
			client.Close()
			if agent != nil {
				agent.Close()
			}
		}
	}()
	if err := client.SetDeadline(time.Now().Add(shortTimeout)); err != nil {
		return
	}
	buffer := make([]byte, 262)
	// ---> ver nMethods methods
	if _, err := io.ReadFull(client, buffer[0:2]); err != nil {
		return
	}
	if buffer[0] != 0x5 {
		return
	}
	nMethods := int(buffer[1])
	methods := buffer[2 : 2+nMethods]
	if _, err := io.ReadFull(client, methods); err != nil {
		return
	}
	// <--- ver method
	method := byte(0xff)
	for _, b := range methods {
		if b == 0x0 {
			method = b
			break
		}
	}
	buffer[1] = method
	_, err := client.Write(buffer[0:2])
	if err != nil || method == 0xff {
		return
	}
	// ---> request
	if _, err := io.ReadFull(client, buffer[0:5]); err != nil {
		return
	}
	if buffer[0] != 0x5 {
		return
	}
	if buffer[1] != 0x1 {
		client.Write(sock5BadReply)
		return
	}
	var (
		addr string
		port []byte
	)
	if atyp := buffer[3]; atyp == 0x3 { // domain
		n := int(buffer[4])
		if _, err := io.ReadFull(client, buffer[5:7+n]); err != nil {
			return
		}
		addr = string(buffer[5 : 5+n])
		port = buffer[5+n : 7+n]
	} else if atyp == 0x1 { // ipv4
		if _, err := io.ReadFull(client, buffer[5:10]); err != nil {
			return
		}
		addr = net.IP(buffer[4:8]).String()
		port = buffer[8:10]
	} else if atyp == 0x4 { // ipv6
		if _, err := io.ReadFull(client, buffer[5:22]); err != nil {
			return
		}
		addr = net.IP(buffer[4:28]).String()
		port = buffer[20:22]
	} else {
		client.Write(sock5BadReply)
		return
	}
	addr += ":" + strconv.FormatUint((uint64(port[0])<<8)|uint64(port[1]), 10)
	// dial agent
	conn, err := net.DialTimeout("tcp", target, shortTimeout)
	if err != nil {
		return
	}
	agent = conn.(*net.TCPConn)
	// write link
	if err := agent.SetDeadline(time.Now().Add(shortTimeout)); err != nil {
		return
	}
	unit := newUnit(secret)
	unit.forEncrypt(reqHead)
	copy(unit.space(), addr)
	if err := unit.writeTo(agent, unitKindLink, len(addr)); err != nil {
		return
	}
	// read ok
	unit.forDecrypt()
	kind, _, err := unit.readFrom(agent)
	if err != nil || kind != unitKindOK {
		return
	}
	// <--- reply
	if _, err := client.Write(sock5GoodReply); err != nil {
		return
	}
	// free unit
	unit = nil

	linked = true

	// now relay
	var state closeState
	state.setFrom(client)
	go tcpEncryptRelay(client, agent, reqHead, &state, secret)
	tcpDecryptRelay(agent, client, &state, secret)
}
