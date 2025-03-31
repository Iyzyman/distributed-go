// server/state.go
package main

import (
    "math/rand"
    "net"
    "sync"
    "time"

    "github.com/Iyzyman/distributed-go/common"
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

// Booking holds detailed info about one booking
type Booking struct {
    ConfirmationID string

    // Start time
    StartDay    uint8 // 0=Monday..6=Sunday
    StartHour   uint8 // 0..23
    StartMinute uint8 // 0..59

    // End time
    EndDay    uint8 // 0=Monday..6=Sunday
    EndHour   uint8 // 0..23
    EndMinute uint8 // 0..59
    Participants []string
}

// FacilityInfo stores everything about one facility
type FacilityInfo struct {
    Name     string
    Bookings []Booking
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

    // Seed random for demonstration (e.g. for generating booking IDs)
    rand.Seed(time.Now().UnixNano())

    // Seed some example facilities & bookings
    srv.facilityData["RoomA"] = &FacilityInfo{
        Name: "RoomA",
        Bookings: []Booking{
            {
                ConfirmationID: "BKG-10000",
                StartDay:       0, // Monday
                StartHour:      9,
                StartMinute:    0,
                EndDay:         0,
                EndHour:        10,
                EndMinute:      0,
            },
            {
                ConfirmationID: "BKG-10001",
                StartDay:       1, // Tuesday
                StartHour:      14,
                StartMinute:    0,
                EndDay:         1,
                EndHour:        15,
                EndMinute:      30,
            },
        },
    }

    srv.facilityData["Lab1"] = &FacilityInfo{
        Name: "Lab1",
        Bookings: []Booking{
            {
                ConfirmationID: "BKG-20000",
                StartDay:       2, // Wednesday
                StartHour:      10,
                StartMinute:    0,
                EndDay:         2,
                EndHour:        12,
                EndMinute:      0,
            },
        },
    }

    return srv
}
