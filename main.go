package main

import (
	"net"
	"fmt"
	"bufio"
	"os"
	"strings"
)

const NOT_LOGIN  = 530
const SUCC_LOGIN = 230
const SUCC_USERNAME = 331
const SERVICE_READY = 220
const PASSV_LNK_READY = 227

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
			commandPrefix: "ls", 
			commandFunction: List,
		},
		commandInfo {
			commandPrefix: "get ", 
			commandFunction: RetrieveFile,
		},
		commandInfo {
			commandPrefix: "stor ", 
			commandFunction: SendFile,
		},
	}

	var d net.Dialer
	var controlConn net.Conn
	var recognizedCommand bool
	buff := make([]byte, 1024)

	reader := bufio.NewReader(os.Stdin)
	for true {
		recognizedCommand = false
		CleanBuffer(buff)
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
				recognizedCommand = true
				break
			}
		}

		if controlConn == nil {
			fmt.Println("No current connection. Connect to host")
		} else if !recognizedCommand && strings.HasPrefix(userInput, "login") {
			Login(controlConn, reader)
		} else if !recognizedCommand {		
			SendFTPcontrolMessage(controlConn, userInput)
		}
	}

}
