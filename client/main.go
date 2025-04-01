package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"time"
	"math/rand"

	"github.com/Iyzyman/distributed-go/client/cli"
)

// Command-line flags for client
var (
    serverAddrFlag = flag.String("serverAddr", "localhost:2222", "Server address in host:port format")
    timeoutFlag    = flag.Int("timeout", 5, "Timeout in seconds for waiting for server replies")
    packetDemoFlag = flag.Bool("packetDemo", false, "If true, simulate packet loss or other network issues")
)

func main() {
	flag.Parse()

	// Parse server address
	serverAddr, err := net.ResolveUDPAddr("udp", *serverAddrFlag)
	if err != nil {
		log.Fatalf("Invalid server address %s: %v", *serverAddrFlag, err)
	}

	// Create UDP socket
	conn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Initialize client state
	client := &cli.ClientState{
		Conn:        conn,
		ServerAddr:  serverAddr,
		Timeout:     time.Duration(*timeoutFlag) * time.Second,
		NextReqID:   uint64(rand.Int63()),
		MonitorMode: false,
		PacketDemo:  *packetDemoFlag,
	}

	fmt.Printf("Connected to server at %s\n", serverAddr)
	if client.PacketDemo {
        fmt.Println("Packet loss simulation is ENABLED (packetDemo=true)")
    } else {
        fmt.Println("Packet loss simulation is DISABLED (packetDemo=false)")
    }
	fmt.Println("Facility Booking System Client")
	fmt.Println("==============================")

	// Start the CLI
	client.RunCLI()
}
