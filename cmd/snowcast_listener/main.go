package main

import (
    "fmt"
    "net"
    "os"
)

const MaxPacketSize = 1500

func main() {
    if len(os.Args) != 2 {
        fmt.Fprintln(os.Stderr, "Usage: snowcast_listener <udp_port>")
        os.Exit(0)
    }

    udpPort := os.Args[1]

    udpAddr, err := net.ResolveUDPAddr("udp4", "localhost:"+udpPort)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error resolving UDP address: %s\n", err)
        os.Exit(0)
    }

    udpConn, err := net.ListenUDP("udp4", udpAddr)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error creating UDP listener: %s\n", err)
        os.Exit(0)
    }
    defer udpConn.Close()

    fmt.Fprintf(os.Stderr, "Listening on UDP port %s...\n", udpPort)

    buffer := make([]byte, MaxPacketSize)

    for {
        n, addr, err := udpConn.ReadFromUDP(buffer)
        if err != nil {
            fmt.Fprintf(os.Stderr, "Error reading UDP packet: %s\n", err)
            continue
        }
		os.Stdout.Write(buffer[:n])
		fmt.Fprintf(os.Stderr, "Received data from %s:%d\n", addr.IP, addr.Port)
    }
}