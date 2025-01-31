package main

import (
	"log"
	"net"
	"fmt"
	"strconv"
	"bufio"
	"os"
	"strings"
	"bytes"
)

type commandInfo struct {
	commandPrefix   string
	commandFunction func(string, net.Dialer, *net.Conn) bool
}

func Login(activeConnection net.Conn, reader *bufio.Reader) bool {
	fmt.Print("username: ")
	userInput := ReadUserInput(reader)

	fmt.Print("password: ")
	passInput := ReadUserInput(reader)

	
	retCode, _ := SendFTPcontrolMessage(activeConnection, "USER " + userInput)

	if retCode != SUCC_USERNAME {
		fmt.Println("Invalid username")
		return false
	}

	retCode, _ = SendFTPcontrolMessage(activeConnection, "PASS " + passInput)

	if retCode != SUCC_LOGIN {
		fmt.Println("Incorrect password/username. Exiting login function")
		return false
	}

	fmt.Println("Correct username/password. Logged in.")

	return true
}

func ReadUserInput(reader *bufio.Reader) string {
	userInput,err := reader.ReadString('\n')
	if err != nil {
		log.Fatalf("Failed to retrieve user input: %v", err)
	}
	return strings.TrimRight(userInput, "\n\r")
}

func CleanBuffer(buff []byte) {
	for i:=range(buff) {
		buff[i] = 0
	}
}

func ConnectFTP(strippedCommand string, dialer net.Dialer, controlConnection *net.Conn) bool {
	var err error
	var buff = make([]byte, 100)
	fmt.Println(strippedCommand)
	*controlConnection, err = dialer.Dial("tcp", strippedCommand)



	if err != nil {	
		fmt.Printf("Failed to dial: %v\n", err)
		return false
	}

	(*controlConnection).Read(buff)
	responseCode,err := strconv.ParseInt(string(buff)[:3], 10, 16)
	if err != nil {
		log.Fatalf("Failed to retrieve error code: %v", err)
	}

	if responseCode == SERVICE_READY {
		fmt.Println("Connection established with " + strippedCommand)
		return true
	} else {
		fmt.Println("Connection could not be established with " + strippedCommand)
		controlConnection = nil
		return false
	}
}

//TODO: better error handling
func List(strippedCommand string, dialer net.Dialer, controlConnection *net.Conn) bool {
	buff := make([]byte, 1024)
	dataConn,err := EstablishDataConnection(*controlConnection, dialer, buff)
	if  err != nil {
		fmt.Println(err)
		return false
	}
	SendFTPcontrolMessage(*controlConnection, "LIST")
	receivedData := ReceiveData(*controlConnection, dataConn, buff)
	fmt.Println("Printing received data...")
	fmt.Println(string(receivedData[:]))
	return true
}

func RetrieveFile(filename string, dialer net.Dialer, controlConnection *net.Conn) bool {
	buff := make([]byte, 1024)
	dataConn, err := EstablishDataConnection(*controlConnection, dialer, buff)
	if  err != nil {
		fmt.Println(err)
		return false
	}
	SendFTPcontrolMessage(*controlConnection, "RETR " + filename)
	receivedData := ReceiveData(*controlConnection, dataConn, buff)
	receivedData = bytes.TrimRight(receivedData, "\x00")
	err = os.WriteFile(filename, receivedData, 0666)
	if err != nil {
		log.Fatal(err)
	}
	return true
}

func SendFile(filename string, dialer net.Dialer, controlConnection *net.Conn) bool {
	buff := make([]byte, 1024)

	fileData,err := os.ReadFile(filename)
	byteLen := len(fileData)
	if  err != nil {
		fmt.Println(err)
		return false
	}

	dataConn, err := EstablishDataConnection(*controlConnection, dialer, buff)
	if  err != nil {
		fmt.Println(err)
		return false
	}
	
	code, msg := SendFTPcontrolMessage(*controlConnection, "STOR " + filename)
	if code != 150 { //TODO make this a define
		fmt.Println(msg)
		return false
	}

	dataConn.Write(fileData)
	dataConn.Close()
	(*controlConnection).Read(buff)
	fmt.Println(buff)
	fmt.Println("sent " + strconv.Itoa(byteLen) + " bytes of data")
	return true
}
