// server/main.go
package main

import (
    "flag"
    "log"
    "net"
    "strings"
)

// Command-line flags for server
var (
    portFlag       = flag.Int("port", 2222, "UDP port to listen on")
    semanticsFlag  = flag.String("semantics", SemanticsAtLeastOnce, "Invocation semantics: at-least-once or at-most-once")
)

func main() {
    flag.Parse()

    semantics := strings.ToLower(*semanticsFlag)
    if semantics != SemanticsAtLeastOnce && semantics != SemanticsAtMostOnce {
        log.Fatalf("Unknown semantics: %s. Choose '%s' or '%s'.",
            semantics, SemanticsAtLeastOnce, SemanticsAtMostOnce)
    }

    // Create the server state
    srv := NewServerState(semantics)

    // Listen on UDP
    addr := net.UDPAddr{IP: net.ParseIP("0.0.0.0"), Port: *portFlag}
    conn, err := net.ListenUDP("udp", &addr)
    if err != nil {
        log.Fatalf("Failed to listen on UDP port %d: %v", *portFlag, err)
    }
    defer conn.Close()

    // Attach the connection to the server state so it can send replies/callbacks
    srv.conn = conn

    log.Printf("Server listening on UDP %s with semantics=%s\n",
        conn.LocalAddr().String(), semantics)

    // Read loop
    buf := make([]byte, 2048)
    for {
        n, clientAddr, err := conn.ReadFromUDP(buf)
        if err != nil {
            log.Printf("ReadFromUDP error: %v\n", err)
            continue
        }

        // Handle in a goroutine if you want concurrency
        go srv.handlePacket(buf[:n], clientAddr)
    }
}
