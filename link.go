package main

import (
	"net"
	"sync"
	"time"

	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
)

const (
	defaultSecret = "0123456789abcdef"
	shortTimeout  = time.Minute
	longTimeout   = 30 * time.Minute
)

type closeState struct {
	mutex         sync.Mutex
	from          *net.TCPConn
	frReadClosed  bool
	frWriteClosed bool
	toReadClosed  bool
	toWriteClosed bool
}

func (cs *closeState) setFrom(from *net.TCPConn) { cs.from = from }

func (cs *closeState) closeSide(from *net.TCPConn, to *net.TCPConn) {
	cs.mutex.Lock()
	from.CloseRead()
	to.CloseWrite()
	if from == cs.from {
		cs.frReadClosed = true
		cs.toWriteClosed = true
		if cs.frWriteClosed {
			from.Close()
		}
		if cs.toReadClosed {
			to.Close()
		}
	} else {
		cs.toReadClosed = true
		cs.frWriteClosed = true
		if cs.toWriteClosed {
			to.Close()
		}
		if cs.frReadClosed {
			from.Close()
		}
	}
	cs.mutex.Unlock()
}

func tcpRelay(from *net.TCPConn, to *net.TCPConn, state *closeState) {
	var (
		buffer = make([]byte, 8192)
		dLen   int
		frErr  error
		toErr  error
	)
	for {
		// read data
		if frErr = from.SetReadDeadline(time.Now().Add(longTimeout)); frErr != nil {
			goto checkErr
		}
		if dLen, frErr = from.Read(buffer); dLen > 0 {
			// write data
			if toErr = to.SetWriteDeadline(time.Now().Add(longTimeout)); toErr != nil {
				goto checkErr
			}
			_, toErr = to.Write(buffer[:dLen])
		}
	checkErr:
		if frErr != nil || toErr != nil {
			state.closeSide(from, to)
			break
		}
	}
}

func roundUp(n int, size int) int {
	r := (n / size) * size
	if n == r {
		return r
	}
	return r + size
}

func xorBytes(dst []byte, a []byte, b []byte) {
	for i := 0; i < len(a); i++ {
		dst[i] = a[i] ^ b[i]
	}
}

const (
	unitKindData = 0
	unitKindLink = 1
	unitKindOK   = 2
	unitKindErr  = 3
	unitKindPing = 4
	unitKindAuth = 5
)

type unit struct {
	buffer []byte
	secKey []byte
	block  cipher.Block
	random uint64
}

func newUnit(secret string) *unit {
	u := new(unit)
	u.buffer = make([]byte, 8192)
	u.secKey = []byte(defaultSecret)
	if len(secret) < 16 {
		copy(u.secKey, secret)
	} else {
		copy(u.secKey, secret[:16])
	}
	u.block, _ = aes.NewCipher(u.secKey)
	return u
}

var (
	reqHead = []byte("POST / HTTP/1.1\r\nContent-Length:    \r\n\r\n")
	resHead = []byte("HTTP/1.1 200 OK\r\nContent-Length:    \r\n\r\n")
)

func (u *unit) forEncrypt(head []byte) {
	copy(u.buffer, head)
	u.random = uint64(time.Now().UnixNano())
}

func (u *unit) forDecrypt() {
}

func (u *unit) calcSize() {
	copy(u.buffer[32:36], "    ")
	size := 48 + int(binary.BigEndian.Uint16(u.buffer[72:74]))
	nDiv := 1000
	pass := false
	for i := 32; i < 36; i++ {
		if b := byte(size / nDiv); b > 0 {
			u.buffer[i] = b + '0'
		}
		if u.buffer[i] != ' ' {
			pass = true
		} else if pass {
			u.buffer[i] = '0'
		}
		size %= nDiv
		nDiv /= 10
	}
}

func (u *unit) writeAuthTo(conn *net.TCPConn) error {
	copy(u.space(), u.secKey)
	return u.writeTo(conn, unitKindAuth, 16)
}

func (u *unit) space() []byte { return u.buffer[104:8168] }

func (u *unit) writeTo(conn *net.TCPConn, kind uint16, dLen int) error {
	// encrypt
	binary.BigEndian.PutUint64(u.buffer[88:96], u.random)
	binary.BigEndian.PutUint64(u.buffer[96:104], u.random)
	space := u.space()
	aLen := roundUp(dLen, 16)
	if aLen > 0 {
		u.cbcEncrypt(space[0:aLen], space[0:aLen], u.buffer[88:104])
	}
	// build head
	binary.BigEndian.PutUint16(u.buffer[72:74], uint16(16+aLen))
	binary.BigEndian.PutUint16(u.buffer[74:76], uint16(dLen))
	binary.BigEndian.PutUint16(u.buffer[76:78], kind)
	binary.BigEndian.PutUint16(u.buffer[78:80], 0)
	binary.BigEndian.PutUint64(u.buffer[80:88], 0)
	// build hmac
	edge := 104 + aLen
	copy(u.buffer[edge:], u.secKey)
	sum := sha256.Sum256(u.buffer[72 : edge+16])
	copy(u.buffer[40:72], sum[:])
	// write conn
	u.calcSize()
	_, err := conn.Write(u.buffer[0:edge])
	return err
}

func (u *unit) cbcEncrypt(dst []byte, src []byte, iv []byte) {
	for len(src) > 0 {
		xorBytes(dst[:16], src[:16], iv)
		u.block.Encrypt(dst[:16], dst[:16])
		iv = dst[:16]
		src = src[16:]
		dst = dst[16:]
	}
}

var (
	errBadProtocol = errors.New("bad protocol")
	errDataInvalid = errors.New("data invalid")
)

func (u *unit) readFrom(conn *net.TCPConn) (kind uint16, data []byte, err error) {
	// read conn
	if _, err = io.ReadFull(conn, u.buffer[0:88]); err != nil {
		return
	}
	// parse head
	bLen := int(binary.BigEndian.Uint16(u.buffer[72:74]))
	if bLen > 8080 {
		err = errBadProtocol
		return
	}
	edge := 88 + bLen
	if _, err = io.ReadFull(conn, u.buffer[88:edge]); err != nil {
		return
	}
	// check hmac
	copy(u.buffer[edge:], u.secKey)
	if sum := sha256.Sum256(u.buffer[72 : edge+16]); !bytes.Equal(sum[:], u.buffer[40:72]) {
		err = errDataInvalid
		return
	}
	// decrypt
	if edge > 104 {
		u.cbcDecrypt(u.buffer[104:edge], u.buffer[104:edge], u.buffer[88:104])
	}
	kind = binary.BigEndian.Uint16(u.buffer[76:78])
	if edge = 104 + int(binary.BigEndian.Uint16(u.buffer[74:76])); edge > 104 {
		data = u.buffer[104:edge]
	} else {
		data = nil
	}
	return
}

func (u *unit) cbcDecrypt(dst []byte, src []byte, iv []byte) {
	end := len(src)
	start := end - 16
	prev := start - 16
	for start > 0 {
		u.block.Decrypt(dst[start:end], src[start:end])
		xorBytes(dst[start:end], dst[start:end], src[prev:start])
		end = start
		start = prev
		prev -= 16
	}
	u.block.Decrypt(dst[start:end], src[start:end])
	xorBytes(dst[start:end], dst[start:end], iv)
}

func tcpEncryptRelay(from *net.TCPConn, to *net.TCPConn, head []byte, state *closeState, secret string) {
	var (
		unit  = newUnit(secret)
		space = unit.space()
		dLen  int
		frErr error
		toErr error
	)
	unit.forEncrypt(head)
	for {
		// read
		if frErr = from.SetReadDeadline(time.Now().Add(longTimeout)); frErr != nil {
			goto checkErr
		}
		if dLen, frErr = from.Read(space); dLen > 0 {
			// write
			if toErr = to.SetWriteDeadline(time.Now().Add(longTimeout)); toErr != nil {
				goto checkErr
			}
			toErr = unit.writeTo(to, unitKindData, dLen)
		}
	checkErr:
		if frErr != nil || toErr != nil {
			state.closeSide(from, to)
			break
		}
	}
}

func tcpDecryptRelay(from *net.TCPConn, to *net.TCPConn, state *closeState, secret string) {
	var (
		unit  = newUnit(secret)
		kind  uint16
		data  []byte
		frErr error
		toErr error
	)
	unit.forDecrypt()
	for {
		// read
		if frErr = from.SetReadDeadline(time.Now().Add(longTimeout)); frErr != nil {
			goto checkErr
		}
		kind, data, frErr = unit.readFrom(from)
		if frErr != nil {
			goto checkErr
		}
		if kind == unitKindPing {
			continue
		}
		// write
		if toErr = to.SetWriteDeadline(time.Now().Add(longTimeout)); toErr != nil {
			goto checkErr
		}
		_, toErr = to.Write(data)

	checkErr:
		if frErr != nil || toErr != nil {
			state.closeSide(from, to)
			break
		}
	}
}
