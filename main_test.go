package main

import (
	"encoding/binary"
	"net"
	"strconv"
	"testing"
	"time"
)

//TODO implement this to provide real testing
type connTester struct {
	deadline time.Time
}

func (c *connTester) Read(b []byte) (n int, err error) {
	return 0, nil
}

func (c *connTester) SetDeadline(t time.Time) error {
	c.deadline = t
	return nil
}

func (c *connTester) ReadFromUDP(buffer []byte) (byteCount int, addr *net.UDPAddr, err error) {

	return 0, nil, nil
}

//These are minimal tests I used to verify parsing a few of the operation types
func TestParseAck(test *testing.T) {
	bytes := make([]byte, 4)
	binary.BigEndian.PutUint16(bytes[0:], ACK)
	binary.BigEndian.PutUint16(bytes[2:], 50)
	request := NewRequest(bytes, 4, nil)
	if request.opcode != ACK {
		test.Errorf("Should have been an ACK but was: %d", request.opcode)
	}
	if request.blockNum != 50 {
		test.Errorf("Blocknum should have been 50 but was: %d", request.blockNum)
	}
}

func TestParseErr(test *testing.T) {
	bytes := make([]byte, 20)
	binary.BigEndian.PutUint16(bytes[0:], ERR)
	binary.BigEndian.PutUint16(bytes[2:], DISK_FULL)
	str := ""
	for i := 4; i < 20; i++ {
		digit := strconv.Itoa(i)
		str += digit
		bytes[i] = digit[0]
	}
	bytes[19] = 0
	request := NewRequest(bytes, 20, nil)
	if request.opcode != ERR {
		test.Errorf("Should have been an ACK but was: %d", request.opcode)
	}
	if request.errorCode != DISK_FULL {
		test.Errorf("errorCode should have been DISK_FULL but was: %d", request.errorCode)
	}
	if request.errorMessage != string(bytes[4:19]) {
		test.Errorf("errorMessage should have been %s but was: %s", str, request.errorMessage)
	}
}
