// server/ops.go
package main

import (
    "fmt"
    "log"
    "net"

    "../common"
)

// handlePacket is called from main.go whenever a packet arrives
func (s *ServerState) handlePacket(data []byte, clientAddr *net.UDPAddr) {
    // 1) Unmarshal the request
    reqMsg, err := common.UnmarshalRequest(data)
    if err != nil {
        log.Printf("Failed to unmarshal request from %s: %v", clientAddr, err)
        return
    }

    // 2) Build a RequestKey for dedup (at-most-once only)
    key := RequestKey{
        Addr:      clientAddr.String(),
        RequestID: reqMsg.RequestID,
    }

    // 3) Check for duplicate if semantics = at-most-once
    if s.semantics == SemanticsAtMostOnce {
        s.historyLock.Lock()
        cachedReply, found := s.history[key]
        s.historyLock.Unlock()

        if found {
            // Duplicate request, just resend old reply
            log.Printf("Duplicate request %d from %s -> resending cached reply",
                reqMsg.RequestID, clientAddr)
            rawReply, marshalErr := common.MarshalReply(cachedReply)
            if marshalErr == nil {
                s.conn.WriteToUDP(rawReply, clientAddr)
            }
            return
        }
    }

    // 4) Process the operation
    reply := s.processOperation(reqMsg, clientAddr)

    // 5) Store in history if at-most-once
    if s.semantics == SemanticsAtMostOnce {
        s.historyLock.Lock()
        s.history[key] = reply
        s.historyLock.Unlock()
    }

    // 6) Marshal and send the reply
    rawReply, err := common.MarshalReply(reply)
    if err != nil {
        log.Printf("Error marshalling reply: %v", err)
        return
    }
    s.conn.WriteToUDP(rawReply, clientAddr)
}

// processOperation dispatches to the correct handler based on OpCode
func (s *ServerState) processOperation(req common.RequestMessage, clientAddr *net.UDPAddr) common.ReplyMessage {
    rep := common.ReplyMessage{
        RequestID: req.RequestID,
        OpCode:    req.OpCode,
        Status:    0,
        Data:      "",
    }

    switch req.OpCode {

    case common.OpQueryAvailability:
        dataStr := s.handleQuery(req.FacilityName, req.DaysList)
        rep.Data = dataStr

    case common.OpBookFacility:
        msg, status := s.handleBookFacility(req)
        rep.Data = msg
        rep.Status = status

    case common.OpChangeBooking:
        msg, status := s.handleChangeBooking(req)
        rep.Data = msg
        rep.Status = status

    case common.OpMonitorAvailability:
        msg, status := s.handleMonitorRegistration(clientAddr, req)
        rep.Data = msg
        rep.Status = status

    case common.OpCancelBooking:
        msg, status := s.handleCancelBooking(req)
        rep.Data = msg
        rep.Status = status

    case common.OpAddParticipant:
        msg, status := s.handleAddParticipant(req)
        rep.Data = msg
        rep.Status = status

    default:
        rep.Status = -1
        rep.Data = fmt.Sprintf("Unknown OpCode %d", req.OpCode)
    }

    return rep
}
