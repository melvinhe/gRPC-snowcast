package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	CommandHello     = 0
	CommandStation   = 1
	CommandWelcome   = 2
	CommandAnnounce  = 3
	CommandInvalid   = 4
	MaxPacketSize    = 1500
	SongDataRate     = 16 * 1024 // 16 KiB/s (16384)
	TimeoutThreshold = 100 * time.Millisecond
)

type Station struct {
	ID        uint16
	Name      string
	SongFile  string
	Listeners []net.UDPAddr
}

type ClientInfo struct {
	UDPAddr        net.UDPAddr
	CurrentStation int
}

var (
	stations       []Station
	clientsMutex   sync.Mutex
	listenersMutex sync.Mutex
	announceMutex  sync.Mutex
	stationMutex   = make(map[uint16]*sync.Mutex)
	clients        = make(map[net.Conn]ClientInfo)
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "User input: snowcast_server <listen port> <file0> [file 1] [file 2] ...")
		os.Exit(0)
	}

	listenPort := os.Args[1]
	files := os.Args[2:]

	// Initialize stations
	for i, file := range files {
		// Construct the absolute path to the song file
		absPath, err := filepath.Abs(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting absolute path for %s: %s\n", file, err)
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "Opening song file: %s\n", absPath)
		newStation := Station{
			ID:        uint16(i),
			Name:      file,    // Use the file name as the station name
			SongFile:  absPath, // Set the absolute path to the song file
			Listeners: []net.UDPAddr{},
		}
		stations = append(stations, newStation)
		// go routine (take in station, stream to each of listeners)
		// need some kind of mutex to access the Listeners array
		stationMutex[newStation.ID] = &sync.Mutex{}
		go streamSongs(newStation)

	}

	tcpAddr, err := net.ResolveTCPAddr("tcp4", "localhost:"+listenPort)
	if err != nil {
		panic(err)
	}

	listenConn, err := net.ListenTCP("tcp4", tcpAddr)
	if err != nil {
		panic(err)
	}
	defer listenConn.Close()

	fmt.Fprintln(os.Stderr, "Snowcast server is running on TCP port "+listenPort)

	// Create a channel to handle OS signals (e.g., Ctrl+C)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Start a goroutine to listen for the 'p' or 'q' input
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			input := scanner.Text()
			if input == "q" {
				listenConn.Close()
				os.Exit(0)
			} else if input == "p" {
				printStations()
			} else if strings.HasPrefix(input, "p ") {
				fileName := strings.TrimSpace(strings.TrimPrefix(input, "p "))
				saveStationsToFile(fileName)
			}
		}
	}()

	// Start a goroutine to listen for Ctrl+C and gracefully shut down the server
	go func() {
		<-sigCh
		listenConn.Close()
		os.Exit(0)
	}()

	for {
		clientConn, err := listenConn.AcceptTCP()
		if err != nil {
			if clientConn != nil {
				fmt.Fprintf(os.Stderr, "Error accepting client connection: %s\n", err)
			}
			continue
		}
		fmt.Fprintln(os.Stderr, "Client Connected from:", clientConn.RemoteAddr())
		go handleClient(clientConn)
	}
}

func handleClient(clientConn net.Conn) {
	defer func() {
		clientConn.Close()
		clientsMutex.Lock()
		delete(clients, clientConn)
		clientsMutex.Unlock()
	}()

	helloMessage := make([]byte, 3)
	clientConn.SetReadDeadline(time.Now().Add(TimeoutThreshold))
	// Read the first byte (command type)
	_, err := clientConn.Read(helloMessage[:1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading command type from server!: %s\n", err)
		clientConn.Close()
		return
	}
	clientConn.SetReadDeadline(time.Time{})
	if helloMessage[0] != CommandHello {
		fmt.Fprintf(os.Stderr, "Welcome error response bad\n")
		clientConn.Close()
		return
	}
	commandType := helloMessage[0]
	// Read the numStations field, which is sent across multiple packets
	udpPortBytes := make([]byte, 2)
	bytesRead := 0
	clientConn.SetReadDeadline(time.Now().Add(TimeoutThreshold))
	for bytesRead < 2 {
		// Read one byte at a time
		n, err := clientConn.Read(udpPortBytes[bytesRead : bytesRead+1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "baddy response: %d\n", commandType)
			clientConn.Close()
			return
		}
		bytesRead += n
	}
	clientConn.SetReadDeadline(time.Time{})
	udpPort := binary.BigEndian.Uint16(udpPortBytes)
    fmt.Fprintf(os.Stderr, "baddy response: %d\n", udpPort)
	udpAddr := &net.UDPAddr{
		IP:   clientConn.RemoteAddr().(*net.TCPAddr).IP,
		Port: int(udpPort),
	}

	// Send Welcome message to the client
	numStations := uint16(len(stations))
	welcomeMessage := make([]byte, 3)
	welcomeMessage[0] = CommandWelcome
	binary.BigEndian.PutUint16(welcomeMessage[1:], numStations)
	_, err = clientConn.Write(welcomeMessage)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error sending Welcome message: %s\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "Sent Welcome message to client. NumStations: %d\n", numStations)
	// Register the client's UDP address
	clientsMutex.Lock()
	clients[clientConn] = ClientInfo{
		UDPAddr:        *udpAddr,
		CurrentStation: -1, // Initialize to an invalid station
	}
	clientsMutex.Unlock()

	// Read client SetStation
	readClient(clientConn)
}

func readClient(clientConn net.Conn) {
	for {
		buffer := make([]byte, 3)
		clientConn.SetReadDeadline(time.Now().Add(TimeoutThreshold))

		// Read the first byte (command type)
		_, err := clientConn.Read(buffer[:1])
		if err != nil {
			if err == io.EOF {
				//clientConn.SetReadDeadline(time.Time{})
				clientsMutex.Lock()
				clientInfo, _ := clients[clientConn]
				//clientsMutex.Unlock()
				previousStation := &stations[clientInfo.CurrentStation]
				//clientsMutex.Lock()
				removeListener(previousStation, clientInfo.UDPAddr)
				clientsMutex.Unlock()
			}
			continue
		}
		commandType := buffer[0]

		switch commandType {
		case CommandStation:
			// Read the numStations field, which is sent across multiple packets
			numStationsBytes := make([]byte, 2)
			bytesRead := 0
			for bytesRead < 2 {
				// Read one byte at a time
				//clientConn.SetReadDeadline(time.Now().Add(TimeoutThreshold))
				n, err := clientConn.Read(numStationsBytes[bytesRead : bytesRead+1])
				if err != nil {
					errorMsg := fmt.Sprintf("Invalid set station command")
					fmt.Fprintln(os.Stderr, errorMsg)
					sendInvalidCommandToClient(clientConn, errorMsg)
					clientConn.SetReadDeadline(time.Time{})
					return
				}
				bytesRead += n
			}
			clientConn.SetReadDeadline(time.Time{})
			stationNumber := binary.BigEndian.Uint16(numStationsBytes)
			if int(stationNumber) < len(stations) {
				go handleStationSwitch(clientConn, stationNumber)
			} else {
				errorMsg := fmt.Sprintf("Invalid station number (doesn't exist): %d", stationNumber)
				fmt.Fprintln(os.Stderr, errorMsg)
				sendInvalidCommandToClient(clientConn, errorMsg)
			}
		case CommandHello:
			fmt.Fprintf(os.Stderr, "Too many hellos!\n")
			sendInvalidCommandToClient(clientConn, "Hello shouldn't be sent more than once")
		default:
			// Handle invalid command types and out of bounds commands
			fmt.Fprintf(os.Stderr, "Unexpected command type from client\n")
			sendInvalidCommandToClient(clientConn, "Unexpected command type")
		}
	}
}

func handleStationSwitch(clientConn net.Conn, stationNumber uint16) {
	clientsMutex.Lock()
	clientInfo, ok := clients[clientConn]
	clientsMutex.Unlock()
	if !ok {
		fmt.Fprintf(os.Stderr, "Client info not found for %s\n", clientConn.RemoteAddr())
		return
	}
	if clientInfo.CurrentStation == int(stationNumber) {
		return
	}

	stationMutex[stationNumber].Lock() // Lock the station being switched to
	defer stationMutex[stationNumber].Unlock()

	newStation := &stations[stationNumber]
	if clientInfo.CurrentStation >= 0 {
		// Remove the client from the previous station's listeners
		previousStation := &stations[clientInfo.CurrentStation]
		removeListener(previousStation, clientInfo.UDPAddr)
	}
	listenersMutex.Lock()
	// Add the client to the new station's listeners
	newStation.Listeners = append(newStation.Listeners, clientInfo.UDPAddr)
	listenersMutex.Unlock()
	// Update the current station for the client
	clientsMutex.Lock()
	clients[clientConn] = ClientInfo{
		UDPAddr:        clientInfo.UDPAddr,
		CurrentStation: int(stationNumber),
	}
	clientsMutex.Unlock()

	fmt.Fprintf(os.Stderr, "Client %s switched to station %d: %s\n", clientConn.RemoteAddr(), stationNumber, newStation.Name)
	sendAnnounceToClient(clientConn, newStation.Name)
}

func sendAnnounceToClient(clientConn net.Conn, songFile string) {
	// Send an Announce message to indicate the new song
	announceMessage := makeAnnounceMessage(songFile)
	_, err := clientConn.Write(announceMessage)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error sending Announce message: %s\n", err)
	}
	fmt.Fprintf(os.Stderr, "Sent Announce message: %s\n", announceMessage)
}

func sendAnnounceToClients(songStation Station) {
	// Create an Announce message to indicate the new song
	stationName := songStation.Name
	announceMessage := makeAnnounceMessage(stationName)
	// Send the Announce message to all connected clients
	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	for clientConn, clientInfo := range clients {
		if clientInfo.CurrentStation == int(songStation.ID) {
			_, err := clientConn.Write(announceMessage)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error sending Announce message to client: %s\n", err)
			}
            fmt.Fprintf(os.Stderr, "Sent Announce message: %s\n", announceMessage)
		}
	}
}

func makeAnnounceMessage(songName string) []byte {
	message := make([]byte, 2+len(songName))
	message[0] = CommandAnnounce
	message[1] = uint8(len(songName))
	copy(message[2:], songName)
	return message
}

func printStations() {
	for _, station := range stations {
		stationInfo := []byte(fmt.Sprintf("%d,%s", station.ID, station.Name))
		os.Stdout.Write(stationInfo)
		listenersMutex.Lock()
		for _, listener := range station.Listeners {
			listenerInfo := []byte(fmt.Sprintf(",%s:%d", listener.IP.String(), listener.Port))
			os.Stdout.Write(listenerInfo)
		}
		listenersMutex.Unlock()
		fmt.Println()
	}
}

func saveStationsToFile(fileName string) {
	file, err := os.Create(fileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating file: %s\n", err)
		return
	}
	defer file.Close()
	for _, station := range stations {
		line := fmt.Sprintf("%d,%s", station.ID, station.Name)
		listenersMutex.Lock()
		for _, listener := range station.Listeners {
			line += fmt.Sprintf(",%s:%d", listener.IP.String(), listener.Port)
		}
		listenersMutex.Unlock()
		line += "\n"
		_, err := file.WriteString(line)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing to file: %s\n", err)
			return
		}
	}
	fmt.Fprintf(os.Stderr, "Stations saved to %s\n", fileName)
}

func sendInvalidCommandToClient(clientConn net.Conn, errorMsg string) {
	fmt.Fprintf(os.Stderr, "Sending invalid command to client")
	errorMessage := make([]byte, 2+len(errorMsg))
	errorMessage[0] = CommandInvalid
	errorMessage[1] = uint8(len(errorMsg))
	copy(errorMessage[2:], errorMsg)
	_, err := clientConn.Write(errorMessage)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error sending InvalidCommand message: %s\n", err)
	}
	clientConn.Close()
}

func removeListener(station *Station, listenerAddr net.UDPAddr) {
	for i, addr := range station.Listeners {
		if addr.IP.Equal(listenerAddr.IP) && addr.Port == listenerAddr.Port {
			// Remove the listener by swapping with the last element and truncating the slice
			listenersMutex.Lock()
			station.Listeners[i] = station.Listeners[len(station.Listeners)-1]
			station.Listeners = station.Listeners[:len(station.Listeners)-1]
			listenersMutex.Unlock()
			return
		}
	}
}

func streamSongs(streamStation Station) {
    // Open the song file
    file, err := os.Open(streamStation.SongFile)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error opening song file for station %s: %s\n", streamStation.Name, err)
        return
    }
    // Calculate t_chunk based on the desired streaming rate (16 KiB/s)
    chunksPerSecond := SongDataRate / MaxPacketSize
    t_chunk := time.Second / time.Duration(chunksPerSecond)
    for {
        // Read a chunk from the file
        chunk := make([]byte, MaxPacketSize)
        bytesRead, err := file.Read(chunk)
        if err != nil {
            if err == io.EOF {
                // Reached the end of the song, send an Announce message and reset the file pointer
                sendAnnounceToClients(streamStation)
                file.Seek(0, 0) // Reset the file pointer to the beginning
                continue
            } else {
                fmt.Fprintf(os.Stderr, "Error reading song file for station %s: %s\n", streamStation.Name, err)
                return
            }
        }
        // Send the chunk to all connected listeners
        listenersMutex.Lock()
        for _, station := range stations {
            if station.ID == streamStation.ID {
                for _, listener := range station.Listeners {
                        addrString := fmt.Sprintf("%s:%d", "localhost", listener.Port)
            
                        resolveUDPAddr, err := net.ResolveUDPAddr("udp4", addrString)
                        if err != nil {
                            fmt.Fprintf(os.Stderr, "error resolving %s\n", err)
                            continue
                        }
                        udpConn, err := net.DialUDP("udp4", nil, resolveUDPAddr)
                        if err != nil {
                            fmt.Fprintf(os.Stderr, "Error creating UDP connection to listener: %s\n", err)
                            continue
                        }
                        // Send the chunk to the listener
                        _, err = udpConn.Write(chunk[:bytesRead])
                        if err != nil {
                            fmt.Fprintf(os.Stderr, "Error sending chunk to listener: %s\n", err)
                        }
                }
            }
        }
        listenersMutex.Unlock()
        time.Sleep(t_chunk)
	}
}
