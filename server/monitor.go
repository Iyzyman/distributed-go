// server/monitor.go
package main

import (
    "fmt"
    "net"
    "time"

    "../common"
)

// handleMonitorRegistration registers the client for callbacks
func (s *ServerState) handleMonitorRegistration(clientAddr *net.UDPAddr, req common.RequestMessage) (string, int32) {
    // Check facility existence
    s.dataLock.Lock()
    _, ok := s.facilityData[req.FacilityName]
    s.dataLock.Unlock()
    if !ok {
        return fmt.Sprintf("Facility '%s' not found", req.FacilityName), -1
    }

    duration := req.MonitorPeriod
    expiry := time.Now().Add(time.Duration(duration) * time.Second)

    reg := MonitorRegistration{
        ClientAddr:   clientAddr,
        FacilityName: req.FacilityName,
        ExpiresAt:    expiry,
    }

    s.monitorLock.Lock()
    s.monitorSubs = append(s.monitorSubs, reg)
    s.monitorLock.Unlock()

    return fmt.Sprintf("Monitoring %s for %d seconds.", req.FacilityName, duration), 0
}

// notifySubscribers is called whenever a facility’s schedule changes
func (s *ServerState) notifySubscribers(facility, updateMsg string) {
    now := time.Now()

    s.monitorLock.Lock()
    defer s.monitorLock.Unlock()

    newSubs := make([]MonitorRegistration, 0, len(s.monitorSubs))
    for _, sub := range s.monitorSubs {
        if sub.FacilityName == facility && now.Before(sub.ExpiresAt) {
            // Build a callback reply
            cb := common.ReplyMessage{
                RequestID: 0,      // no direct request ID for callback
                OpCode:    100,    // or any "callback" code
                Status:    0,
                Data:      fmt.Sprintf("Facility=%s updated: %s", facility, updateMsg),
            }

            raw, err := common.MarshalReply(cb)
            if err == nil {
                s.conn.WriteToUDP(raw, sub.ClientAddr)
            }

            newSubs = append(newSubs, sub)
        } else if now.Before(sub.ExpiresAt) {
            // Different facility or not expired
            newSubs = append(newSubs, sub)
        }
        // else it's expired, skip
    }

    // Clean up expired
    s.monitorSubs = newSubs
}
