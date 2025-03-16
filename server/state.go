// server/state.go
package main

import (
    "math/rand"
    "net"
    "sync"
    "time"

    "path/to/yourproject/common"
)

// Constants for invocation semantics
const (
    SemanticsAtLeastOnce = "at-least-once"
    SemanticsAtMostOnce  = "at-most-once"
)

// RequestKey identifies a (clientAddr, requestID) pair for deduplication
type RequestKey struct {
    Addr      string
    RequestID uint64
}

// FacilityInfo is an example structure for storing data about a facility.
type FacilityInfo struct {
    Name     string
    Bookings []string // Minimal example; you'd store real booking times/IDs
}

// MonitorRegistration holds callback info for a monitoring client
type MonitorRegistration struct {
    ClientAddr   *net.UDPAddr
    FacilityName string
    ExpiresAt    time.Time
}

// ServerState holds all the data the server needs to operate
type ServerState struct {
    semantics string              // "at-least-once" or "at-most-once"
    conn      *net.UDPConn        // For sending replies/callbacks

    // Deduplication history for at-most-once
    history     map[RequestKey]common.ReplyMessage
    historyLock sync.Mutex

    // Facility data (in-memory store)
    facilityData map[string]*FacilityInfo
    dataLock     sync.Mutex

    // Monitoring subscriptions
    monitorSubs []MonitorRegistration
    monitorLock sync.Mutex
}

// NewServerState initializes everything
func NewServerState(semantics string) *ServerState {
    srv := &ServerState{
        semantics:    semantics,
        history:      make(map[RequestKey]common.ReplyMessage),
        facilityData: make(map[string]*FacilityInfo),
        monitorSubs:  make([]MonitorRegistration, 0),
    }

    // Seed some example facilities
    srv.facilityData["RoomA"] = &FacilityInfo{Name: "RoomA"}
    srv.facilityData["Lab1"]  = &FacilityInfo{Name: "Lab1"}

    // Seed random for demonstration (e.g. for generating booking IDs)
    rand.Seed(time.Now().UnixNano())

    return srv
}
