package main

import (
	"log"
	"net"
	"fmt"
	"encoding/binary"
	"unsafe"
	"strconv"
	"bufio"
	"os"
	"strings"
	"bytes"
)

var networkOrder binary.ByteOrder = binary.BigEndian

const NOT_LOGIN  = 530
const SUCC_LOGIN = 230
const SUCC_USERNAME = 331
const SERVICE_READY = 220
const PASSV_LNK_READY = 227

func findEndian() binary.ByteOrder {
	var i int = 0x0100
	ptr := unsafe.Pointer(&i)
	if 0x01 == *(*byte)(ptr) {
	    return binary.BigEndian
	} else if 0x00 == *(*byte)(ptr) {
	    return binary.LittleEndian
	} else {
		log.Fatalf("Failed to determine local system endianness")
	}

	return networkOrder // default to make compiler happy
}

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
			fmt.Println("appending...")
			receivedData = append(receivedData, buff...)

		}
	}

	return nil
}

func SendFTPcontrolMessage(tcpConnection net.Conn, message string, buff []byte) (int, string) {
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
func EstablishDataConnection(commandConnection net.Conn, dialer net.Dialer, buff[]byte) net.Conn {
	var ip [6] int
	retCode, retMsg := SendFTPcontrolMessage(commandConnection, "PASV", buff)

	if retCode != PASSV_LNK_READY {
		return nil
	}

	_, err := fmt.Sscanf(retMsg, "Entering Passive Mode (%d,%d,%d,%d,%d,%d).", &ip[0], &ip[1], &ip[2], &ip[3], &ip[4], &ip[5])
    if err != nil {
        fmt.Println(err)
	}
	//TODO: Make this line more readable. It sucks ass right now
	addrString := strconv.Itoa(ip[0]) + "." +strconv.Itoa(ip[1])+"." +strconv.Itoa(ip[2])+"." +strconv.Itoa(ip[3]) +":"+strconv.Itoa(ip[4]*256 + ip[5])
	dataConn, err := dialer.Dial("tcp", addrString)
	if err == nil {
		return dataConn
	}

	log.Fatalf("Failed to dial: %v", err)
	return nil
}

func Login(activeConnection net.Conn, reader *bufio.Reader, buff []byte) bool {
	fmt.Print("username: ")
	userInput := ReadUserInput(reader)

	fmt.Print("password: ")
	passInput := ReadUserInput(reader)

	
	retCode, _ := SendFTPcontrolMessage(activeConnection, "USER " + userInput, buff)

	if retCode != SUCC_USERNAME {
		fmt.Println("Invalid username")
		return false
	}

	retCode, _ = SendFTPcontrolMessage(activeConnection, "PASS " + passInput, buff)

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

func main() {
	var dataConn net.Conn
	var d net.Dialer
	buff := make([]byte, 1024)
	
	controlConn, err := d.Dial("tcp", "10.0.0.7:21")
	if err != nil {
		log.Fatalf("Failed to dial: %v", err)
	}
	defer controlConn.Close()

	if err != nil {
		log.Fatalf("Failed to dial: %v", err)
	}

	for true{
		controlConn.Read(buff)
		if buff[0] == '2' && buff[1] == '2' && buff[2] == '0' {
			fmt.Println("successfull connection")
			break
		}
	}

	reader := bufio.NewReader(os.Stdin)
	for true {
		CleanBuffer(buff)
		fmt.Print("Enter command: ")
		userInput := ReadUserInput(reader)
		if strings.ToLower(userInput) == "login" {
			Login(controlConn, reader, buff)
		} else if strings.ToLower(userInput) == "ls" {
			dataConn = EstablishDataConnection(controlConn, d, buff)
			SendFTPcontrolMessage(controlConn, "list", buff)
			receivedData := ReceiveData(controlConn, dataConn, buff)
			fmt.Println("Printing received data...")
			fmt.Println(string(receivedData[:]))
		} else if strings.HasPrefix(userInput, "retr ") {
			filename := strings.TrimPrefix(userInput, "retr ")
			dataConn = EstablishDataConnection(controlConn, d, buff)
			SendFTPcontrolMessage(controlConn, userInput, buff)
			receivedData := ReceiveData(controlConn, dataConn, buff)
			receivedData = bytes.TrimRight(receivedData, "\x00")
			err := os.WriteFile(filename, receivedData, 0666)
			if err != nil {
				log.Fatal(err)
			}
		} else if strings.HasPrefix(userInput, "stor ") {
			filename := strings.TrimPrefix(userInput, "stor ")
			dataConn = EstablishDataConnection(controlConn, d, buff)
			SendFTPcontrolMessage(controlConn, userInput, buff)
			fileData,err := os.ReadFile(filename)
			if err != nil {
				log.Fatal(err)
			}
			dataConn.Write(fileData)
			dataConn.Close()
			controlConn.Read(buff)
		} else {
			fmt.Println(SendFTPcontrolMessage(controlConn, userInput, buff))
		}
	}

	fmt.Println(SendFTPcontrolMessage(controlConn, "PASS bftp", buff))

	fmt.Println(SendFTPcontrolMessage(controlConn, "QUIT", buff))
}
