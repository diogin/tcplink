package main

import (
	"net"
	"time"
)

func serveBroker(args map[string]string) {
	secret, listen, target := args["secret"], args["listen"], args["target"]
	next := make(chan bool, 4)
	for i := 0; i < 4; i++ {
		go relayBroker(next, secret, listen, target)
	}
	for {
		<-next
		go relayBroker(next, secret, listen, target)
	}
}

func relayBroker(next chan bool, secret string, listen string, target string) {
	var (
		conn   net.Conn
		err    error
		router *net.TCPConn
		server *net.TCPConn
		kind   uint16
	)
	unit := newUnit(secret)
start:
	if router != nil {
		router.Close()
	}
	// dial router
	conn, err = net.DialTimeout("tcp", target, shortTimeout)
	if err != nil {
		time.Sleep(time.Second)
		goto start
	}
	router = conn.(*net.TCPConn)
	// write auth
	if err = router.SetDeadline(time.Now().Add(shortTimeout)); err != nil {
		goto start
	}
	unit.forEncrypt(reqHead)
	if err = unit.writeAuthTo(router); err != nil {
		goto start
	}
	// read ok
	unit.forDecrypt()
	if kind, _, err = unit.readFrom(router); err != nil || kind != unitKindOK {
		goto start
	}
	next <- true
	// dial server
	conn, err = net.DialTimeout("tcp", listen, shortTimeout)
	if err != nil {
		router.Close()
		return
	}
	server = conn.(*net.TCPConn)
	// free unit
	unit = nil

	// now relay
	var state closeState
	state.setFrom(server)
	go tcpEncryptRelay(server, router, reqHead, &state, secret)
	tcpDecryptRelay(router, server, &state, secret)
}
