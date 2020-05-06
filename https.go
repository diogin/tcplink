package main

import (
	"bufio"
	"bytes"
	"net"
	"net/url"
	"strings"
	"time"
)

func serveHttps(listen string, target string, secret string) {
	addr, err := net.ResolveTCPAddr("tcp", listen)
	must(err)
	door, err := net.ListenTCP("tcp", addr)
	must(err)
	for {
		if client, err := door.AcceptTCP(); err == nil {
			go relayHttps(client, target, secret)
		}
	}
}

func relayHttps(client *net.TCPConn, target string, secret string) {
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
	if https {
		// <--- response
		if _, err := client.Write(http200); err != nil {
			return
		}
	} else {
		// write the data read
		unit.forEncrypt(reqHead)
		copy(unit.space(), buffer) // space is confirmed to be large enough
		if err := unit.writeTo(agent, unitKindData, len(buffer)); err != nil {
			return
		}
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
