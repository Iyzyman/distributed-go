// server/ops.go
package main

import (
    "fmt"
    "log"
    "net"
    "strconv"
    "time"

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

// intersectsDays returns true if a booking touches any of the input days
func intersectsDays(bk Booking, days []uint8) bool {
    for _, d := range days {
        // if the booking starts at day bk.StartDay and ends at day bk.EndDay
        // check if d is in [StartDay..EndDay] (naive approach)
        if d >= bk.StartDay && d <= bk.EndDay {
            return true
        }
    }
    return false
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


// handleQuery returns a string listing bookings on the specified days for the facility
func (s *ServerState) handleQuery(name string, days []uint8) string {
    s.dataLock.Lock()
    fac, ok := s.facilityData[name]
    s.dataLock.Unlock()
    if !ok {
        return fmt.Sprintf("Error: Facility '%s' not found", name)
    }

    // If days is empty, we might return all bookings
    if len(days) == 0 {
        return s.listAllBookings(fac)
    }

    // Build a response with just the bookings that intersect those days
    result := fmt.Sprintf("Facility=%s availability for days=%v:\n", name, days)
    for _, bk := range fac.Bookings {
        // If any day in [bk.StartDay..bk.EndDay] overlaps with 'days', we show it
        if intersectsDays(bk, days) {
            result += fmt.Sprintf("  - %s: Day %d (%02d:%02d) to Day %d (%02d:%02d)\n",
                bk.ConfirmationID,
                bk.StartDay, bk.StartHour, bk.StartMinute,
                bk.EndDay, bk.EndHour, bk.EndMinute,
            )
        }
    }
    return result
}

// timesOverlap returns true if [start1, end1) intersects [start2, end2).
func timesOverlap(start1, end1, start2, end2 int32) bool {
    // Overlap occurs if the start of one range is before the end of the other, and vice versa
    return (start1 < end2) && (start2 < end1)
}

// toAbsoluteMinutes converts (day, hour, minute) to an absolute minute count from Monday 0:00.
// e.g., Monday 9:00 -> day=0, hour=9 => 9 * 60 = 540
func toAbsoluteMinutes(day, hour, minute uint8) int32 {
    return int32(day)*24*60 + int32(hour)*60 + int32(minute)
}


// handleBookFacility creates a new booking if no overlap.
func (s *ServerState) handleBookFacility(req common.RequestMessage) (string, int32) {
    facName := req.FacilityName

    // Acquire lock to read/modify facility data
    s.dataLock.Lock()
    defer s.dataLock.Unlock()

    fac, ok := s.facilityData[facName]
    if !ok {
        return fmt.Sprintf("Facility '%s' not found", facName), -1
    }

    // Convert requested start/end to absolute minutes
    newStart := toAbsoluteMinutes(req.StartDay, req.StartHour, req.StartMinute)
    newEnd   := toAbsoluteMinutes(req.EndDay, req.EndHour, req.EndMinute)
    if newEnd <= newStart {
        return "Error: End time must be after start time.", -1
    }

    // Check for collision with existing bookings
    for _, bk := range fac.Bookings {
        existingStart := toAbsoluteMinutes(bk.StartDay, bk.StartHour, bk.StartMinute)
        existingEnd   := toAbsoluteMinutes(bk.EndDay, bk.EndHour, bk.EndMinute)
        if timesOverlap(newStart, newEnd, existingStart, existingEnd) {
            // There's a conflict
            return "Time conflict with an existing booking.", 1
        }
    }

    // If no conflict, create a new booking
    newID := fmt.Sprintf("BKG-%d", time.Now().UnixNano())
    newBooking := Booking{
        ConfirmationID: newID,
        StartDay:       req.StartDay,
        StartHour:      req.StartHour,
        StartMinute:    req.StartMinute,
        EndDay:         req.EndDay,
        EndHour:        req.EndHour,
        EndMinute:      req.EndMinute,
    }
    fac.Bookings = append(fac.Bookings, newBooking)

    // Notify monitoring clients that facility updated
    s.notifySubscribers(facName, fmt.Sprintf("New booking created: %s", newID))

    msg := fmt.Sprintf("Booked '%s' from Day %d (%02d:%02d) to Day %d (%02d:%02d). ID=%s",
        facName,
        req.StartDay, req.StartHour, req.StartMinute,
        req.EndDay, req.EndHour, req.EndMinute,
        newID,
    )
    return msg, 0
}


// handleChangeBooking locates the booking by ConfirmationID and shifts by OffsetMinutes
func (s *ServerState) handleChangeBooking(req common.RequestMessage) (string, int32) {
    confID   := req.ConfirmationID
    newStart := toAbsoluteMinutes(req.StartDay, req.StartHour, req.StartMinute)
    newEnd   := toAbsoluteMinutes(req.EndDay, req.EndHour, req.EndMinute)
    if newEnd <= newStart {
        return "Error: End time must be after start time.", -1
    }

    s.dataLock.Lock()
    defer s.dataLock.Unlock()

    var oldBooking *Booking
    var oldFac *FacilityInfo
    var oldIndex int
    var facName string

    // 1) Find the old booking
    for fName, facility := range s.facilityData {
        for i, bk := range facility.Bookings {
            if bk.ConfirmationID == confID {
                oldBooking = &bk
                oldIndex   = i
                oldFac     = facility
                facName    = fName
                break
            }
        }
        if oldBooking != nil {
            break
        }
    }
    if oldBooking == nil {
        return fmt.Sprintf("Error: Booking %s not found", confID), -1
    }

    // Preserve old participants before we remove the booking
    oldParticipants := oldBooking.Participants

    // 2) Remove the old booking from the list
    oldFac.Bookings = append(oldFac.Bookings[:oldIndex], oldFac.Bookings[oldIndex+1:]...)

    // 3) Check collision for new times
    for _, bk := range oldFac.Bookings {
        start := toAbsoluteMinutes(bk.StartDay, bk.StartHour, bk.StartMinute)
        end   := toAbsoluteMinutes(bk.EndDay, bk.EndHour, bk.EndMinute)
        if timesOverlap(newStart, newEnd, start, end) {
            // revert old booking
            oldFac.Bookings = append(oldFac.Bookings, *oldBooking)
            return "Time conflict with an existing booking.", 1
        }
    }

    // 4) Recreate the updated booking, reusing participants
    updated := Booking{
        ConfirmationID: confID,
        StartDay:       req.StartDay,
        StartHour:      req.StartHour,
        StartMinute:    req.StartMinute,
        EndDay:         req.EndDay,
        EndHour:        req.EndHour,
        EndMinute:      req.EndMinute,
        Participants:   oldParticipants, // Keep old list of participants
    }
    oldFac.Bookings = append(oldFac.Bookings, updated)

    // 5) Notify subscribers
    s.notifySubscribers(facName,
        fmt.Sprintf("Booking %s changed to Day %d(%02d:%02d) -> Day %d(%02d:%02d)",
            confID,
            req.StartDay, req.StartHour, req.StartMinute,
            req.EndDay, req.EndHour, req.EndMinute))

    return fmt.Sprintf("Changed booking %s to new times, no conflict.", confID), 0
}


// handleMonitorRegistration just adds a subscription entry
func (s *ServerState) handleMonitorRegistration(clientAddr *net.UDPAddr, req common.RequestMessage) (string, int32) {
    facName := req.FacilityName

    s.dataLock.Lock()
    _, ok := s.facilityData[facName]
    s.dataLock.Unlock()

    if !ok {
        return fmt.Sprintf("Facility '%s' not found", facName), -1
    }

    duration := req.MonitorPeriod
    expiry := time.Now().Add(time.Duration(duration) * time.Second)

    sub := MonitorRegistration{
        ClientAddr:   clientAddr,
        FacilityName: facName,
        ExpiresAt:    expiry,
    }

    s.monitorLock.Lock()
    s.monitorSubs = append(s.monitorSubs, sub)
    s.monitorLock.Unlock()

    return fmt.Sprintf("Monitoring %s for %d seconds.", facName, duration), 0
}

// handleCancelBooking is idempotent: removing the same booking twice is harmless
func (s *ServerState) handleCancelBooking(req common.RequestMessage) (string, int32) {
    confID := req.ConfirmationID

    s.dataLock.Lock()
    defer s.dataLock.Unlock()

    for facName, fac := range s.facilityData {
        for i, bk := range fac.Bookings {
            if bk.ConfirmationID == confID {
                // Remove it
                fac.Bookings = append(fac.Bookings[:i], fac.Bookings[i+1:]...)
                s.notifySubscribers(facName, fmt.Sprintf("Booking %s canceled", confID))
                return fmt.Sprintf("Canceled booking %s", confID), 0
            }
        }
    }

    // If we didn't find it, it's either already canceled or never existed
    return fmt.Sprintf("Booking %s not found (already canceled?)", confID), 0
}

// handleAddParticipant is the non-idempotent example
// We'll just log the action. If repeated, it "re-adds" a participant.
func (s *ServerState) handleAddParticipant(req common.RequestMessage) (string, int32) {
    confID := req.ConfirmationID
    participant := req.ParticipantName

    s.dataLock.Lock()
    defer s.dataLock.Unlock()

    var foundBooking *Booking
    var facName string

    // Locate the booking in any facility
    for fn, fac := range s.facilityData {
        for i := range fac.Bookings {
            if fac.Bookings[i].ConfirmationID == confID {
                foundBooking = &fac.Bookings[i] // pointer to the booking in slice
                facName = fn
                break
            }
        }
        if foundBooking != nil {
            break
        }
    }
    if foundBooking == nil {
        return fmt.Sprintf("Error: Booking %s not found", confID), -1
    }

    // Append participant
    // This is non-idempotent because repeated calls will keep adding the same name
    foundBooking.Participants = append(foundBooking.Participants, participant)

    // Notify watchers
    s.notifySubscribers(facName,
        fmt.Sprintf("Participant %s added to booking %s", participant, confID))

    return fmt.Sprintf("Added participant=%s to booking=%s", participant, confID), 0
}

// -----------------------------------------------------------------------------
// Helper functions
// -----------------------------------------------------------------------------

// A trivial function that returns a summary of all bookings for the facility
func (s *ServerState) listAllBookings(fac *FacilityInfo) string {
    if len(fac.Bookings) == 0 {
        return fmt.Sprintf("Facility=%s has no bookings.", fac.Name)
    }
    result := fmt.Sprintf("Facility=%s, existing bookings:\n", fac.Name)
    for _, bk := range fac.Bookings {
        result += fmt.Sprintf("  - %s: Day %d (%02d:%02d) to Day %d (%02d:%02d)\n",
            bk.ConfirmationID,
            bk.StartDay, bk.StartHour, bk.StartMinute,
            bk.EndDay, bk.EndHour, bk.EndMinute,
        )
        if len(bk.Participants) > 0 {
            result += fmt.Sprintf("      Participants: %v\n", bk.Participants)
        }
    }
    return result
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

