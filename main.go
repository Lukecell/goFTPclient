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

const NOT_LOGIN  = 530
const SUCC_LOGIN = 230
const SUCC_USERNAME = 331
const SERVICE_READY = 220
const PASSV_LNK_READY = 227

type commandInfo struct {
	commandPrefix   string
	commandFunction func(string, net.Dialer, *net.Conn) bool
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
	SendFTPcontrolMessage(*controlConnection, "LIST", buff)
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
	SendFTPcontrolMessage(*controlConnection, "RETR " + filename, buff)
	receivedData := ReceiveData(*controlConnection, dataConn, buff)
	receivedData = bytes.TrimRight(receivedData, "\x00")
	err = os.WriteFile(filename, receivedData, 0666)
	if err != nil {
		log.Fatal(err)
	}
	return true
}

func main() {
	var validCommands = []commandInfo {
		commandInfo {
			commandPrefix: "open ", 
			commandFunction: ConnectFTP,
		},
		commandInfo {
			commandPrefix: "list", 
			commandFunction: List,
		},
		commandInfo {
			commandPrefix: "get ", 
			commandFunction: RetrieveFile,
		},
	}


//	var dataConn net.Conn
	var d net.Dialer
	var controlConn net.Conn
	buff := make([]byte, 1024)
/*	
	controlConn, err := d.Dial("tcp", "10.0.0.7:21")
	if err != nil {
		log.Fatalf("Failed to dial: %v", err)
	}
	defer controlConn.Close()

	if err != nil {
	}

	for true{
		controlConn.Read(buff)
		if buff[0] == '2' && buff[1] == '2' && buff[2] == '0' {
			fmt.Println("successfull connection")
			break
		}
	}
	*/

//	loggedIn := false
	reader := bufio.NewReader(os.Stdin)
	for true {
		CleanBuffer(buff)
//		commandExecuted := false
		fmt.Print("Enter command: ")
		userInput := ReadUserInput(reader)

		if userInput=="quit" || userInput == "q" {
			fmt.Println("quitting")
			if controlConn != nil {
				controlConn.Close()
			}

			return
		}

		for _,command := range validCommands {
			if strings.HasPrefix(userInput, command.commandPrefix){
				strippedCommand := strings.TrimPrefix(userInput, command.commandPrefix)
				command.commandFunction(strippedCommand, d, &controlConn)
//				commandExecuted = false
				break
			}
		}

		if controlConn == nil {
			fmt.Println("No current connection. Connect to host")
		}


		if strings.ToLower(userInput) == "login" {
			Login(controlConn, reader, buff)
		} /*else if strings.ToLower(userInput) == "ls" {
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
		}*/
	}

}
