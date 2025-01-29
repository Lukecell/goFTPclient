package main

import (
	"log"
	"net"
	"fmt"
	"strconv"
)

func ReceiveData(commandConn net.Conn, dataConn net.Conn, buff []byte) []byte {
	var receivedData []byte

	for true {
		CleanBuffer(buff)
		_,err := dataConn.Read(buff)

		//fmt.Println(buff)

		if err != nil {
			commandConn.Read(buff)
			fmt.Println(string(buff[:]))

			return receivedData
		} else {
			receivedData = append(receivedData, buff...)

		}
	}

	return nil
}

func SendFTPcontrolMessage(tcpConnection net.Conn, message string, buff []byte) (int, string) {
	if tcpConnection == nil {
		return NOT_LOGIN, ""
	}
	// FTP receives commands in ascii byte form, terminated by \r\n
	outputMessage := make([]byte,len(message) + 2)
	for i:=0; i<len(message); i++ {
		outputMessage[i] = message[i]
	}
	outputMessage[len(message)] = '\r'
	outputMessage[len(message)+1] = '\n'

	tcpConnection.Write(outputMessage)
	// get response...
	for true {
		_,err := tcpConnection.Read(buff)

		responseMessage := string(buff)

		if err != nil {
			log.Fatalf("Failed to read message: %v", err)
		}
		responseCode,err := strconv.ParseInt(responseMessage[:3], 10, 16)
		if err != nil {
			log.Fatalf("Failed to process return code: %v", err)
		}

		return int(responseCode), responseMessage[4:]
	}

	return -1, "SOMETHING WENT WRONG. TERMINATE THE PROGRAM!"
}

/*
* Establishes an FTP data connection using passive mode
*/
func EstablishDataConnection(controlConnection net.Conn, dialer net.Dialer, buff[]byte) (net.Conn, error){
	var ip [6] int
	retCode, retMsg := SendFTPcontrolMessage(controlConnection, "PASV", buff)

	if retCode != PASSV_LNK_READY {
		return nil, fmt.Errorf("%v", retMsg)
	}

	_, err := fmt.Sscanf(retMsg, "Entering Passive Mode (%d,%d,%d,%d,%d,%d).", &ip[0], &ip[1], &ip[2], &ip[3], &ip[4], &ip[5])
    if err != nil {
        fmt.Println(err)
	}
	//TODO: Make this line more readable. It sucks ass right now
	addrString := strconv.Itoa(ip[0]) + "." +strconv.Itoa(ip[1])+"." +strconv.Itoa(ip[2])+"." +strconv.Itoa(ip[3]) +":"+strconv.Itoa(ip[4]*256 + ip[5])
	dataConn, err := dialer.Dial("tcp", addrString)
	if err == nil {
		return dataConn, nil
	}

	log.Fatalf("Failed to dial: %v", err)
	return nil, fmt.Errorf("Failed to connect to server's data port. Hell if I know why")
}
