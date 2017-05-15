package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"time"
)

// Request a struct which encapsulates request information
// If we had more request types, I'd want to break these out and build a RequestFactory
type Request struct {
	client       *net.UDPAddr
	mode         uint16
	filename     string
	opcode       uint16
	blockNum     uint16
	errorMessage string
	errorCode    uint16
	data         []byte
}

//GetConnection gets the connection from a Request based on the client
func (request Request) GetConnection() *net.UDPConn {
	local, err := net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		panic(err)
	}
	remote, err := net.DialUDP("udp", local, request.client)
	if err != nil {
		panic(err)
	}
	return remote
}

//NewRequest parses a byte array and returns a Request struct
func NewRequest(bytesReceived []byte, byteCount int, remote *net.UDPAddr) *Request {
	opcode := binary.BigEndian.Uint16(bytesReceived)
	request := &Request{opcode: opcode, client: remote}
	bytesReceived = bytesReceived[2:byteCount] // pull off the first 2 opcode bytes
	switch opcode {
	case READ, WRITE:
		//mode is unused for now
		filename, _ := parseReadWriteRequest(bytesReceived)
		request.filename = filename
		return request
	case ERR:
		errorMessage, errorCode := parseError(bytesReceived)
		request.errorMessage = errorMessage
		request.errorCode = errorCode
		return request
	case DATA:
		request.blockNum = binary.BigEndian.Uint16(bytesReceived)
		request.data = bytesReceived[2:]
		return request
	case ACK:
		request.blockNum = binary.BigEndian.Uint16(bytesReceived)
		return request
	}
	//If the opcode did not match any of those operations, return an illegal_operation
	request.errorMessage = "Illegal operation type"
	request.errorCode = ILLEGAL_OPERATION
	return request
}

func parseError(bodyData []byte) (errorMessage string, errorCode uint16) {
	errorCode = binary.BigEndian.Uint16(bodyData)
	bodyData = bodyData[2:]
	//error message is terminated with a null byte, so trim that off the end
	for index, bite := range bodyData {
		if bite == 0 {
			errorMessage = string(bodyData[:index])
		}
	}
	return errorMessage, errorCode
}

//Given the body of a RRQ/WRQ operation (with the opcode removed), retrieve the filename and mode
func parseReadWriteRequest(bodyData []byte) (filename string, mode string) {
	nullBytesFound := 0
	lastNullByteIndex := 0
	for index, bite := range bodyData {
		if bite == 0 {
			//There are 2 null bytes in a R/W request, after the filename and after the mode
			if nullBytesFound == 0 {
				filename = string(bodyData[:index])
				log.Printf("Got filename: %s.", filename)
				lastNullByteIndex = index + 1
				nullBytesFound++
			} else if nullBytesFound == 1 {
				mode = string(bodyData[lastNullByteIndex:index])
				log.Printf("Got mode: %s.", mode)
				nullBytesFound++
				lastNullByteIndex = index + 1
			} else {
				log.Printf("Invalid request body")
				return "", ""
			}
		}
	}
	return filename, mode
}

//File is a struct which encapsulates the information about a File in memory
type File struct {
	completed  bool
	writing    bool
	fileChunks map[uint16][]byte
}

const (
	//from https://tools.ietf.org/html/rfc1350
	//READ Read Request opcode
	READ uint16 = 1
	//WRITE Write Request opcode
	WRITE uint16 = 2
	//DATA Data opcode
	DATA uint16 = 3
	//ACK Acknowledged opcode
	ACK uint16 = 4
	//ERR Error opcode
	ERR uint16 = 5

	//Error codes from appendix to RFC 1350
	UNK               = 0
	NOT_FOUND         = 1
	NO_ACCESS         = 2
	DISK_FULL         = 3
	ILLEGAL_OPERATION = 4
	UNKNOWN_XFER_ID   = 5
	FILE_EXISTS       = 6
	NO_USER_ACCESS    = 7

	BUFFER_SIZE = 1024

	BLOCKSIZE = 512

	BYTE           = 1.0
	KILOBYTE       = 1024 * BYTE
	MEGABYTE       = 1024 * KILOBYTE
	GIGABYTE       = 1024 * MEGABYTE
	MAX_CACHE_SIZE = 1 * MEGABYTE
)

var (
	fileCache map[string]*File
	cacheSize int
)

//HandleWriteRequest handles a request to add a new file to the server
func HandleWriteRequest(request *Request) {

	//We want to keep the connection open for the life of this operation, defer it's closing
	conn := request.GetConnection()
	defer conn.Close()
	filename := request.filename
	if fileCache[filename] != nil {
		if fileCache[filename].writing && !fileCache[filename].completed {
			//If the fileCache contains the file and it's currently being written, don't let it get overwritten
			sendErrorResponse(FILE_EXISTS, "File is currently being written", conn)
			return
		} else if fileCache[filename].completed {
			sendErrorResponse(FILE_EXISTS, "File already exists on server", conn)
			return
		}
		//If we didn't complete and the file is no longer being written, we remove it and let it be re-written
		fileCache[filename] = nil
	}

	log.Println("Got a new file write request")

	//Got a new file
	blockNumber := uint16(0)
	sendAckResponse(blockNumber, conn)
	blockNumber++

	buffer := make([]byte, BUFFER_SIZE)
	fileCache[filename] = &File{completed: false, fileChunks: make(map[uint16][]byte), writing: true}

	//bytesReceived for logging
	bytesReceived := 0

	//retries will be used to retry the current blockNumber.
	retries := 0
	for {
		retries++
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		byteCount, remote, err := conn.ReadFromUDP(buffer)
		if err != nil || retries > 5 {
			fileCache[filename].writing = false
			sendErrorResponse(UNK, "Timeout during write operation", conn)
			return
		}
		request := NewRequest(buffer[:byteCount], byteCount, remote)
		request.client = remote
		if request.opcode == ERR || len(request.errorMessage) > 0 {
			fileCache[filename].writing = false
			sendErrorResponse(request.errorCode, request.errorMessage, conn)
			return
		}
		bytesReceived += len(request.data)

		//Set to 1Mb for now.
		if bytesReceived+cacheSize > MAX_CACHE_SIZE {
			fileCache[filename].writing = false
			sendErrorResponse(DISK_FULL, "Disk is full", conn)
			return
		}

		//Need to check the remote with the request client bc we may have more than once connection at a time
		if request.opcode == DATA &&
			request.blockNum == blockNumber && remote == request.client {
			retries = 0
			log.Println(string(request.data))

			//I probably spent 40% of the homework assignment deciphering why I needed the next 2 lines here.
			//I used the printFileCache(filename) function to figure out that I was just modifying the same buffer over and over again
			data := make([]byte, byteCount-4)
			copy(data, request.data)
			fileCache[filename].fileChunks[blockNumber] = data
			sendAckResponse(blockNumber, conn)
			blockNumber++
		}
		//Blocksize is standard 512b.  Clients can request different sies, but we'll ignore since the protocol does not specify it
		if len(request.data) < BLOCKSIZE {
			log.Printf("Finished receiving file: %s. Bytes received: %d", filename, bytesReceived)
			cacheSize += bytesReceived
			fileCache[filename].writing = false
			fileCache[filename].completed = true
			return
		}
	}
}

//HandleReadRequest handles a RRQ operation.
func HandleReadRequest(request *Request) {

	//We want to keep the connection open for the life of this operation, defer it's closing until the function returns
	conn := request.GetConnection()
	defer conn.Close()
	filename := request.filename

	//If the file is not on the server || is not completed,
	if fileCache[filename] == nil || !fileCache[filename].completed {
		sendErrorResponse(NOT_FOUND, "File not found on server", conn)
		return
	}

	//blockNumber := uint16(1)
	buffer := make([]byte, BUFFER_SIZE)
	bytesSent := 0
	blockNumber := uint16(1)
	for {
		bytes := fileCache[filename].fileChunks[blockNumber]
		if bytes == nil {
			break
		}
		bytesSent += len(bytes)
		sendDataResponse(bytes, blockNumber, conn)

		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		byteCount, remote, err := conn.ReadFromUDP(buffer)
		if err != nil {
			sendErrorResponse(UNK, "Error reading during read operation", conn)
			return
		}
		request := NewRequest(buffer[:byteCount], byteCount, remote)
		if request.opcode == ACK && request.blockNum == blockNumber {
			blockNumber++
		} else if request.opcode == ERR {
			//if NewRequest returns an ERR type, send back to notify the client
			sendErrorResponse(request.errorCode, request.errorMessage, conn)
			return
		} else {
			log.Println("We received an error mid-operation.  Not sure how to recover, so we'll just return for now and talk to product management later")
			return
		}
	}
	log.Printf("Reading file complete: %s. Bytes Sent: %d", filename, bytesSent)
	return
}

//Sends an error response to the given remote client
func sendErrorResponse(errorCode uint16, errorMessage string, remote *net.UDPConn) {
	//ERR is 2 opcode bytes, 2 errCode bytes, 2 null bytes + length of the error message
	errBuff := make([]byte, len([]byte(errorMessage))+6)
	binary.BigEndian.PutUint16(errBuff[0:], ERR)
	binary.BigEndian.PutUint16(errBuff[2:], errorCode)
	copied := copy(errBuff[4:], errorMessage)
	binary.BigEndian.PutUint16(errBuff[4+copied:], 0)
	write(errBuff, remote)
	log.Printf("ERROR - Code: %d. Message: %s", errorCode, errorMessage)
}

//Sends an Ack response to the given remote client
func sendAckResponse(blockNum uint16, remote *net.UDPConn) {
	//ACK is just 4 bytes
	ackBuff := make([]byte, 4)
	binary.BigEndian.PutUint16(ackBuff[0:], ACK)
	binary.BigEndian.PutUint16(ackBuff[2:], blockNum)
	write(ackBuff, remote)
}

//Sends a data response to the given remote client
func sendDataResponse(data []byte, blockNumber uint16, remote *net.UDPConn) {
	dataBuff := make([]byte, 4+len(data))
	binary.BigEndian.PutUint16(dataBuff[0:], DATA)
	binary.BigEndian.PutUint16(dataBuff[2:], blockNumber)
	copy(dataBuff[4:], data)
	write(dataBuff, remote)
}

//writes bytes to a connection
func write(data []byte, remote *net.UDPConn) {
	_, err := remote.Write(data)
	if err != nil {
		log.Println("Error writing to UDPConn data: ", err)
	}
}

//Useful for debugging.
func printFileCache(filename string) {
	var keys []uint16
	for k := range fileCache[filename].fileChunks {
		keys = append(keys, k)
	}
	for _, k := range keys {
		fmt.Println("Key:", k, "Value:", string(fileCache[filename].fileChunks[k]))
	}
}

func main() {
	address, err := net.ResolveUDPAddr("udp", "localhost:6969")
	if err != nil {
		panic(err)
	}
	conn, err := net.ListenUDP("udp", address)
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	buffer := make([]byte, BUFFER_SIZE)
	fileCache = make(map[string]*File)
	cacheSize = 0
	for {
		byteCount, remote, err := conn.ReadFromUDP(buffer)
		if err != nil || byteCount <= 0 {
			log.Printf("Error reading from UDP: %v", err)
			return
		}
		request := NewRequest(buffer[:byteCount], byteCount, remote)
		request.client = remote

		if request.opcode == WRITE {
			go HandleWriteRequest(request)
		}
		if request.opcode == READ {
			go HandleReadRequest(request)
		}
	}
}
