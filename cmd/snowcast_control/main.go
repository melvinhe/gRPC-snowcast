package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

const (
	CommandHello     = 0
	CommandStation   = 1
	CommandWelcome   = 2
	CommandAnnounce  = 3
	CommandInvalid   = 4
	MaxPacketSize    = 1500
	TimeoutThreshold = 100 * time.Millisecond
)

var (
	announceReceived bool
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "User input: snowcast_control <server IP> <server port> <listener port>")
		os.Exit(1)
	}

	serverIP := os.Args[1]
	serverPort := os.Args[2]
	listenerPort := os.Args[3]

	conn, err := net.Dial("tcp4", serverIP+":"+serverPort)
	if err != nil {
		return
	}
	defer conn.Close()

	// Sending Hello message to the server
	helloMessage := make([]byte, 3)
	helloMessage[0] = CommandHello
	listenerPortInt, _ := strconv.Atoi(listenerPort)
	binary.BigEndian.PutUint16(helloMessage[1:], uint16(listenerPortInt))
	_, err = conn.Write(helloMessage)
	if err != nil {
		return
	}
	fmt.Fprintf(os.Stderr, "Sent Hello message\n")

    welcomeMessage := make([]byte, 3)
	conn.SetReadDeadline(time.Now().Add(TimeoutThreshold))
    // Read the first byte (command type)
    _, err = conn.Read(welcomeMessage[:1])
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error reading command type from server!: %s\n", err)
        os.Exit(0)
    }
	conn.SetReadDeadline(time.Time{})
    if welcomeMessage[0] != CommandWelcome {
		fmt.Fprintf(os.Stderr, "Welcome error response bad\n")
		os.Exit(0)
	}
    commandType := welcomeMessage[0]
    // Read the numStations field, which is sent across multiple packets
    numStationsBytes := make([]byte, 2)
    bytesRead := 0
    for bytesRead < 2 {
        // Read one byte at a time
        conn.SetReadDeadline(time.Now().Add(TimeoutThreshold))
        n, err := conn.Read(numStationsBytes[bytesRead : bytesRead+1])
        if err != nil {
            fmt.Fprintf(os.Stderr, "baddy response: %d\n", commandType)
            os.Exit(0)
        }
		conn.SetReadDeadline(time.Time{})
        bytesRead += n
    }
    numStations := binary.BigEndian.Uint16(numStationsBytes)
    welcome := []byte(fmt.Sprintf("Welcome to Snowcast! The server has %d stations.\n", numStations))
	os.Stdout.Write(welcome)
    go readServer(conn)
	// Starting REPL
	REPL(conn, numStations)

}

func REPL(conn net.Conn, numStations uint16) {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Fprintln(os.Stderr, "Type in a number to set the station we're listening to, or 'q' to quit.")
	announceReceived = false
	//fmt.Fprintf(os.Stderr, "> ")
	for {
		//fmt.Fprintf(os.Stderr, "> ")
		scanner.Scan()
		input := scanner.Text()

		if input == "q" {
			break
		}

		stationNumber, err := strconv.Atoi(input)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Invalid input. Please enter a valid station number or 'q' to quit.")
			continue
		}

		// Check if the station number is within the valid range
		if stationNumber < 0 || stationNumber >= int(numStations) {
			fmt.Fprintf(os.Stderr, "Invalid station number. The server has %d stations.\n", numStations)
			//fmt.Fprintf(os.Stderr, "> ")
			continue
		} else {
			fmt.Fprintln(os.Stderr, "Waiting for an announce...")
		}

		// Send SetStation command to the server
		setStationMessage := make([]byte, 3)
		setStationMessage[0] = CommandStation
		binary.BigEndian.PutUint16(setStationMessage[1:], uint16(stationNumber))
		_, err = conn.Write(setStationMessage)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Failed to set station:", err)
			return
		}
		announceReceived = true
	}
}


func readServer(conn net.Conn) {
	for {
		buffer := make([]byte, 1)
		// Read the first byte (command type)
		_, err := conn.Read(buffer[:1])
		if err != nil {
			return
		}
		commandType := buffer[0]

		switch commandType {
		case CommandHello, CommandStation, CommandInvalid, CommandWelcome:
			// Handle invalid command types
			fmt.Fprintf(os.Stderr, "baddy response: %d\n", commandType)
			os.Exit(0)
		case CommandAnnounce:
			if !announceReceived {
				fmt.Fprintf(os.Stderr, "Received ANNOUNCE before SET_STATION. Terminating connection.\n")
				os.Exit(0)
			}

			// Read 1 more byte for the songNameLength field
			conn.SetReadDeadline(time.Now().Add(TimeoutThreshold))
			// Read 1 more byte for the songNameLength field
			songNameLengthBuffer := make([]byte, 1)
			_, err := conn.Read(songNameLengthBuffer)
			if err != nil {
				fmt.Fprintf(os.Stderr, "baddy response: %d\n", commandType)
				os.Exit(0)
			}
			conn.SetReadDeadline(time.Time{})
			songNameLength := int(songNameLengthBuffer[0])
			// Read songNameLength bytes for the songName field
			songName := make([]byte, songNameLength)
			bytesRead := 0
			for bytesRead < songNameLength {
				// Read one byte at a time
				conn.SetReadDeadline(time.Now().Add(TimeoutThreshold))
				n, err := conn.Read(songName[bytesRead : bytesRead+1])
				if err != nil {
					fmt.Fprintf(os.Stderr, "baddy response: %d\n", commandType)
					os.Exit(0)
				}
				conn.SetReadDeadline(time.Time{})
				bytesRead += n
			}
			songNameStr := string(songName)
			announceMessage := []byte(fmt.Sprintf("New song announced: %s\n", songNameStr))
			os.Stdout.Write(announceMessage)
			//fmt.Fprintf(os.Stderr, "> ")

		default:
			// Handle out-of-bounds command types
			fmt.Fprintf(os.Stderr, "baddy response: %d\n", commandType)
			os.Exit(0)
		}
	}
}
