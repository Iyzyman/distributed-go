// server/ops.go
package main

import (
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/Iyzyman/distributed-go/common"
)

// handlePacket is called from main.go whenever a packet arrives
func (s *ServerState) handlePacket(data []byte, clientAddr *net.UDPAddr) {
	log.Printf("Received packet from %s", clientAddr)

	// 1) Unmarshal the request
	reqMsg, err := common.UnmarshalRequest(data)
	if err != nil {
		log.Printf("Failed to unmarshal request from %s: %v", clientAddr, err)
		return
	}
	log.Printf("Unmarshaled request: OpCode=%d, RequestID=%d", reqMsg.OpCode, reqMsg.RequestID)

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
			log.Printf("Duplicate request %d from %s -> resending cached reply", reqMsg.RequestID, clientAddr)
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
	log.Printf("Sending reply for RequestID=%d to %s", reqMsg.RequestID, clientAddr)
	s.conn.WriteToUDP(rawReply, clientAddr)
}

// intersectsDays returns true if a booking touches any of the input days
func intersectsDays(bk Booking, days []uint8) bool {
	for _, d := range days {
		// if the booking starts at day bk.StartDay and ends at day bk.EndDay,
		// check if d is in [StartDay..EndDay] (naive approach)
		if d >= bk.StartDay && d <= bk.EndDay {
			return true
		}
	}
	return false
}

// notifySubscribers is called whenever a facility's schedule changes
func (s *ServerState) notifySubscribers(facility, updateMsg string) {
	now := time.Now()
	log.Printf("Notifying subscribers of facility '%s' update: %s", facility, updateMsg)

	s.monitorLock.Lock()
	defer s.monitorLock.Unlock()

	newSubs := make([]MonitorRegistration, 0, len(s.monitorSubs))
	for _, sub := range s.monitorSubs {
		if sub.FacilityName == facility && now.Before(sub.ExpiresAt) {
			// Build a callback reply
			cb := common.ReplyMessage{
				RequestID: 0,   // no direct request ID for callback
				OpCode:    100, // or any "callback" code
				Status:    0,
				Data:      fmt.Sprintf("Facility=%s updated: %s", facility, updateMsg),
			}
			raw, err := common.MarshalReply(cb)
			if err == nil {
				s.conn.WriteToUDP(raw, sub.ClientAddr)
				log.Printf("Sent callback to %s for facility '%s'", sub.ClientAddr, facility)
			}
			newSubs = append(newSubs, sub)
		} else if now.Before(sub.ExpiresAt) {
			newSubs = append(newSubs, sub)
		}
		// else, subscription expired – do not add
	}
	s.monitorSubs = newSubs
}

// availableTimingsForDay computes available time intervals (as a string)
// for a given day from the list of bookings.
// It clips any booking that spans multiple days to the boundaries of the day.
func availableTimingsForDay(day uint8, bookings []Booking) string {
	dayStart := int32(day) * 1440
	dayEnd := int32(day+1) * 1440

	// Gather bookings that overlap with this day and clip them to day boundaries.
	type interval struct {
		start, end int32
	}
	var dayIntervals []interval
	for _, bk := range bookings {
		// Check if booking intersects the day
		if bk.EndDay < day || bk.StartDay > day {
			continue
		}
		// Convert booking start and end to absolute minutes.
		bkStart := toAbsoluteMinutes(bk.StartDay, bk.StartHour, bk.StartMinute)
		bkEnd := toAbsoluteMinutes(bk.EndDay, bk.EndHour, bk.EndMinute)
		// Clip booking to day boundaries.
		if bkStart < dayStart {
			bkStart = dayStart
		}
		if bkEnd > dayEnd {
			bkEnd = dayEnd
		}
		dayIntervals = append(dayIntervals, interval{bkStart, bkEnd})
	}

	// Sort the intervals by start time.
	for i := 1; i < len(dayIntervals); i++ {
		key := dayIntervals[i]
		j := i - 1
		for j >= 0 && dayIntervals[j].start > key.start {
			dayIntervals[j+1] = dayIntervals[j]
			j--
		}
		dayIntervals[j+1] = key
	}

	// Now compute available intervals.
	available := ""
	current := dayStart
	for _, iv := range dayIntervals {
		if iv.start > current {
			available += fmt.Sprintf("%02d:%02d-%02d:%02d, ", current/60, current%60, iv.start/60, iv.start%60)
		}
		if iv.end > current {
			current = iv.end
		}
	}
	if current < dayEnd {
		available += fmt.Sprintf("%02d:%02d-24:00", current/60, current%60)
	}
	available = strings.TrimSuffix(available, ", ")
	if available == "" {
		available = "Fully booked"
	}
	return available
}

// handleQuery returns a formatted string showing the availability of a facility
// for the specified days. The output is formatted as:
//
//	Day X:
//	  Current bookings:
//	    - <booking details>
//	  Available timings: <free intervals>
func (s *ServerState) handleQuery(name string, days []uint8) string {
	log.Printf("Handling Query for facility '%s' on days %v", name, days)
	s.dataLock.Lock()
	fac, ok := s.facilityData[name]
	s.dataLock.Unlock()
	if !ok {
		log.Printf("Facility '%s' not found during Query", name)
		return fmt.Sprintf("Error: Facility '%s' not found", name)
	}
	result := fmt.Sprintf("Facility %s availability:\n", name)
	for _, day := range days {
		result += fmt.Sprintf("Day %d:\n", day)
		bookingsStr := ""
		for _, bk := range fac.Bookings {
			// Check if the booking intersects the day.
			if bk.StartDay <= day && bk.EndDay >= day {
				bookingsStr += fmt.Sprintf("  - %s: %02d:%02d to %02d:%02d\n",
					bk.ConfirmationID,
					bk.StartHour, bk.StartMinute,
					bk.EndHour, bk.EndMinute,
				)
				if len(bk.Participants) > 0 {
					bookingsStr += fmt.Sprintf("      Participants: %v\n", bk.Participants)
				}
			}
		}
		if bookingsStr == "" {
			bookingsStr = "  None\n"
		}
		result += "Current bookings:\n" + bookingsStr
		avail := availableTimingsForDay(day, fac.Bookings)
		result += "Available timings: " + avail + "\n\n"
	}
	log.Printf("Query result for '%s': %s", name, result)
	return result
}

// timesOverlap returns true if [start1, end1) intersects [start2, end2).
func timesOverlap(start1, end1, start2, end2 int32) bool {
	return (start1 < end2) && (start2 < end1)
}

// toAbsoluteMinutes converts (day, hour, minute) to an absolute minute count from Monday 0:00.
func toAbsoluteMinutes(day, hour, minute uint8) int32 {
	// Convert to absolute minutes from Monday 0:00
	// Each day has 24*60 = 1440 minutes
	return int32(day)*1440 + int32(hour)*60 + int32(minute)
}

// handleBookFacility creates a new booking if no overlap.
func (s *ServerState) handleBookFacility(req common.RequestMessage) (string, int32) {
	facName := req.FacilityName
	log.Printf("Handling BookFacility for facility '%s'", facName)

	s.dataLock.Lock()
	defer s.dataLock.Unlock()

	fac, ok := s.facilityData[facName]
	if !ok {
		log.Printf("Facility '%s' not found in BookFacility", facName)
		return fmt.Sprintf("Facility '%s' not found", facName), -1
	}

	newStart := toAbsoluteMinutes(req.StartDay, req.StartHour, req.StartMinute)
	newEnd := toAbsoluteMinutes(req.EndDay, req.EndHour, req.EndMinute)
	if newEnd <= newStart {
		log.Printf("Invalid booking times: end time is not after start time")
		return "Error: End time must be after start time.", -1
	}

	for _, bk := range fac.Bookings {
		existingStart := toAbsoluteMinutes(bk.StartDay, bk.StartHour, bk.StartMinute)
		existingEnd := toAbsoluteMinutes(bk.EndDay, bk.EndHour, bk.EndMinute)
		if timesOverlap(newStart, newEnd, existingStart, existingEnd) {
			log.Printf("Time conflict detected for facility '%s'", facName)
			return "Time conflict with an existing booking.", 1
		}
	}

	newID := fmt.Sprintf("BKG-%d", time.Now().UnixNano())
	newBooking := Booking{
		ConfirmationID: newID,
		StartDay:       req.StartDay,
		StartHour:      req.StartHour,
		StartMinute:    req.StartMinute,
		EndDay:         req.EndDay,
		EndHour:        req.EndHour,
		EndMinute:      req.EndMinute,
		Participants:   []string{}, // Initially empty
	}
	fac.Bookings = append(fac.Bookings, newBooking)

	s.notifySubscribers(facName, fmt.Sprintf("New booking created: %s", newID))
	msg := fmt.Sprintf("Booked '%s' from Day %d (%02d:%02d) to Day %d (%02d:%02d). ID=%s",
		facName,
		req.StartDay, req.StartHour, req.StartMinute,
		req.EndDay, req.EndHour, req.EndMinute,
		newID,
	)
	log.Printf("Booking successful: %s", msg)
	return msg, 0
}

// fromAbsoluteMinutes converts an absolute minute value to day, hour, and minute.
// For example, if a day has 1440 minutes (24 hours).
func fromAbsoluteMinutes(total int) (uint8, uint8, uint8) {
	day := total / 1440
	rem := total % 1440
	hour := rem / 60
	minute := rem % 60
	return uint8(day), uint8(hour), uint8(minute)
}

// handleChangeBooking locates the booking by ConfirmationID and updates its time using OffsetMinutes.
func (s *ServerState) handleChangeBooking(req common.RequestMessage) (string, int32) {
	offset := req.OffsetMinutes
	confID := req.ConfirmationID
	log.Printf("Handling ChangeBooking for ConfirmationID '%s'", confID)
	log.Printf("Received offset (in minutes): %d", offset)

	s.dataLock.Lock()
	defer s.dataLock.Unlock()

	// Locate the booking using ConfirmationID.
	var oldBooking *Booking
	var oldFac *FacilityInfo
	var oldIndex int
	var facName string

	for fName, facility := range s.facilityData {
		for i, bk := range facility.Bookings {
			if bk.ConfirmationID == confID {
				// Capture a pointer to the found booking.
				oldBooking = &bk
				oldIndex = i
				oldFac = facility
				facName = fName
				break
			}
		}
		if oldBooking != nil {
			break
		}
	}
	if oldBooking == nil {
		log.Printf("Booking '%s' not found in ChangeBooking", confID)
		return fmt.Sprintf("Error: Booking %s not found", confID), -1
	}

	// Convert the current booking's start/end times to absolute minutes.
	oldStart := toAbsoluteMinutes(oldBooking.StartDay, oldBooking.StartHour, oldBooking.StartMinute)
	oldEnd := toAbsoluteMinutes(oldBooking.EndDay, oldBooking.EndHour, oldBooking.EndMinute)
	log.Printf("Old booking times (absolute minutes): start=%d, end=%d", oldStart, oldEnd)

	// Apply the offset to the booking times.
	newStartAbs := oldStart + int32(offset)
	newEndAbs := oldEnd + int32(offset)

	// Validate: the new end time must be after the new start time.
	if newEndAbs <= newStartAbs {
		log.Printf("Invalid new times: new end time (%d) is not after new start time (%d)", newEndAbs, newStartAbs)
		return "Error: End time must be after start time.", -1
	}

	// Convert the new times from absolute minutes back to day, hour, and minute.
	newStartDay, newStartHour, newStartMinute := fromAbsoluteMinutes(int(newStartAbs))
	newEndDay, newEndHour, newEndMinute := fromAbsoluteMinutes(int(newEndAbs))
	log.Printf("New booking times: Start - Day=%d, %02d:%02d; End - Day=%d, %02d:%02d",
		newStartDay, newStartHour, newStartMinute, newEndDay, newEndHour, newEndMinute)

	// Remove the old booking from the facility's booking list.
	oldFac.Bookings = append(oldFac.Bookings[:oldIndex], oldFac.Bookings[oldIndex+1:]...)

	// Check for time collisions with existing bookings.
	for _, bk := range oldFac.Bookings {
		existingStart := toAbsoluteMinutes(bk.StartDay, bk.StartHour, bk.StartMinute)
		existingEnd := toAbsoluteMinutes(bk.EndDay, bk.EndHour, bk.EndMinute)
		if timesOverlap(newStartAbs, newEndAbs, existingStart, existingEnd) {
			// Collision detected; revert removal.
			oldFac.Bookings = append(oldFac.Bookings, *oldBooking)
			log.Printf("Time conflict detected when changing booking '%s'", confID)
			return "Time conflict with an existing booking.", 1
		}
	}

	// Create an updated booking with the new timings.
	updated := Booking{
		ConfirmationID: confID,
		StartDay:       newStartDay,
		StartHour:      newStartHour,
		StartMinute:    newStartMinute,
		EndDay:         newEndDay,
		EndHour:        newEndHour,
		EndMinute:      newEndMinute,
		Participants:   oldBooking.Participants,
	}
	oldFac.Bookings = append(oldFac.Bookings, updated)

	// Notify subscribers of the timing change.
	s.notifySubscribers(facName,
		fmt.Sprintf("Booking %s changed using offset %d min: Day %d (%02d:%02d) -> Day %d (%02d:%02d)",
			confID, offset, newStartDay, newStartHour, newStartMinute, newEndDay, newEndHour, newEndMinute))
	msg := fmt.Sprintf("Changed booking %s by offset %d minutes successfully.", confID, offset)
	log.Printf("ChangeBooking successful: %s", msg)
	return msg, 0
}

// handleMonitorRegistration adds a subscription entry.
func (s *ServerState) handleMonitorRegistration(clientAddr *net.UDPAddr, req common.RequestMessage) (string, int32) {
	facName := req.FacilityName
	log.Printf("Handling MonitorAvailability for facility '%s' from %s", facName, clientAddr)

	s.dataLock.Lock()
	_, ok := s.facilityData[facName]
	s.dataLock.Unlock()
	if !ok {
		log.Printf("Facility '%s' not found in MonitorAvailability", facName)
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

	msg := fmt.Sprintf("Monitoring %s for %d seconds.", facName, duration)
	log.Printf("MonitorRegistration successful: %s", msg)
	return msg, 0
}

// handleCancelBooking removes a booking; idempotent operation.
func (s *ServerState) handleCancelBooking(req common.RequestMessage) (string, int32) {
	confID := req.ConfirmationID
	log.Printf("Handling CancelBooking for ConfirmationID '%s'", confID)

	s.dataLock.Lock()
	defer s.dataLock.Unlock()

	for facName, fac := range s.facilityData {
		for i, bk := range fac.Bookings {
			if bk.ConfirmationID == confID {
				fac.Bookings = append(fac.Bookings[:i], fac.Bookings[i+1:]...)
				s.notifySubscribers(facName, fmt.Sprintf("Booking %s canceled", confID))
				msg := fmt.Sprintf("Canceled booking %s", confID)
				log.Printf("CancelBooking successful: %s", msg)
				return msg, 0
			}
		}
	}

	log.Printf("Booking '%s' not found in CancelBooking (may be already canceled)", confID)
	return fmt.Sprintf("Booking %s not found (already canceled?)", confID), 0
}

// handleAddParticipant appends a participant to a booking; non-idempotent.
func (s *ServerState) handleAddParticipant(req common.RequestMessage) (string, int32) {
	confID := req.ConfirmationID
	participant := req.ParticipantName
	log.Printf("Handling AddParticipant: adding '%s' to booking '%s'", participant, confID)

	s.dataLock.Lock()
	defer s.dataLock.Unlock()

	var foundBooking *Booking
	var facName string
	for fn, fac := range s.facilityData {
		for i := range fac.Bookings {
			if fac.Bookings[i].ConfirmationID == confID {
				foundBooking = &fac.Bookings[i]
				facName = fn
				break
			}
		}
		if foundBooking != nil {
			break
		}
	}
	if foundBooking == nil {
		log.Printf("Booking '%s' not found in AddParticipant", confID)
		return fmt.Sprintf("Error: Booking %s not found", confID), -1
	}

	foundBooking.Participants = append(foundBooking.Participants, participant)
	s.notifySubscribers(facName, fmt.Sprintf("Participant %s added to booking %s", participant, confID))
	msg := fmt.Sprintf("Added participant=%s to booking=%s", participant, confID)
	log.Printf("AddParticipant successful: %s", msg)
	return msg, 0
}

// listAllBookings returns a summary of all bookings for a facility.
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

// processOperation dispatches to the correct handler based on OpCode.
func (s *ServerState) processOperation(req common.RequestMessage, clientAddr *net.UDPAddr) common.ReplyMessage {
	log.Printf("Processing operation with OpCode %d for RequestID %d", req.OpCode, req.RequestID)
	rep := common.ReplyMessage{
		RequestID: req.RequestID,
		OpCode:    req.OpCode,
		Status:    0,
		Data:      "",
	}

	switch req.OpCode {
	case common.OpQueryAvailability:
		rep.Data = s.handleQuery(req.FacilityName, req.DaysList)
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

	log.Printf("Processed RequestID %d with result: %s (Status=%d)", req.RequestID, rep.Data, rep.Status)
	return rep
}
