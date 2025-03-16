package bookingproto

import (
    "encoding/binary"
    "fmt"
)

func MarshalRequest(req RequestMessage) ([]byte, error) {
    // Start with a small buffer
    buf := make([]byte, 0, 128) // adjust as needed

    // 1) OpCode (1 byte)
    buf = append(buf, req.OpCode)

    // 2) RequestID (8 bytes, big-endian)
    tmp := make([]byte, 8)
    binary.BigEndian.PutUint64(tmp, req.RequestID)
    buf = append(buf, tmp...)

    // 3) Switch on OpCode to encode the relevant fields
    switch req.OpCode {

    case OpQueryAvailability:
        // FacilityName
        buf = writeString(buf, req.FacilityName)
        // DaysList: first write 1 byte for number of days, then each day as 1 byte
        if len(req.DaysList) > 255 {
            return nil, fmt.Errorf("too many days in DaysList (max 255)")
        }
        buf = append(buf, byte(len(req.DaysList)))
        for _, d := range req.DaysList {
            buf = append(buf, d)
        }

    case OpBookFacility:
        // FacilityName
        buf = writeString(buf, req.FacilityName)
        // StartDay/Hour/Minute + EndDay/Hour/Minute (6 bytes total)
        buf = append(buf, req.StartDay, req.StartHour, req.StartMinute,
            req.EndDay, req.EndHour, req.EndMinute)

    case OpChangeBooking:
        // ConfirmationID
        buf = writeString(buf, req.ConfirmationID)
        // OffsetMinutes (4 bytes)
        tmp4 := make([]byte, 4)
        binary.BigEndian.PutUint32(tmp4, uint32(req.OffsetMinutes))
        buf = append(buf, tmp4...)

    case OpMonitorAvailability:
        // FacilityName
        buf = writeString(buf, req.FacilityName)
        // MonitorPeriod (4 bytes)
        tmp4 := make([]byte, 4)
        binary.BigEndian.PutUint32(tmp4, req.MonitorPeriod)
        buf = append(buf, tmp4...)

    case OpCancelBooking:
        // ConfirmationID
        buf = writeString(buf, req.ConfirmationID)

    case OpAddParticipant:
        // ConfirmationID
        buf = writeString(buf, req.ConfirmationID)
        // ParticipantName
        buf = writeString(buf, req.ParticipantName)

    default:
        return nil, fmt.Errorf("unknown OpCode %d", req.OpCode)
    }

    return buf, nil
}
func UnmarshalRequest(data []byte) (RequestMessage, error) {
    var req RequestMessage
    offset := 0

    // 1) OpCode (1 byte)
    if len(data) < 1 {
        return req, fmt.Errorf("data too short for opcode")
    }
    req.OpCode = data[offset]
    offset++

    // 2) RequestID (8 bytes)
    if offset+8 > len(data) {
        return req, fmt.Errorf("data too short for requestID")
    }
    req.RequestID = binary.BigEndian.Uint64(data[offset : offset+8])
    offset += 8

    // 3) Switch on OpCode
    switch req.OpCode {

    case OpQueryAvailability:
        // FacilityName
        facName, newOffset, err := readString(data, offset)
        if err != nil {
            return req, err
        }
        req.FacilityName = facName
        offset = newOffset

        // DaysList
        if offset+1 > len(data) {
            return req, fmt.Errorf("not enough bytes for days count")
        }
        ndays := int(data[offset])
        offset++
        if offset+ndays > len(data) {
            return req, fmt.Errorf("not enough bytes for days list")
        }
        req.DaysList = data[offset : offset+ndays]
        offset += ndays

    case OpBookFacility:
        // FacilityName
        facName, newOffset, err := readString(data, offset)
        if err != nil {
            return req, err
        }
        req.FacilityName = facName
        offset = newOffset

        // Next 6 bytes: StartDay/Hour/Minute + EndDay/Hour/Minute
        if offset+6 > len(data) {
            return req, fmt.Errorf("not enough bytes for booking times")
        }
        req.StartDay = data[offset]
        req.StartHour = data[offset+1]
        req.StartMinute = data[offset+2]
        req.EndDay = data[offset+3]
        req.EndHour = data[offset+4]
        req.EndMinute = data[offset+5]
        offset += 6

    case OpChangeBooking:
        // ConfirmationID
        confID, newOffset, err := readString(data, offset)
        if err != nil {
            return req, err
        }
        req.ConfirmationID = confID
        offset = newOffset

        // OffsetMinutes (4 bytes)
        if offset+4 > len(data) {
            return req, fmt.Errorf("not enough bytes for offsetMinutes")
        }
        off := binary.BigEndian.Uint32(data[offset : offset+4])
        req.OffsetMinutes = int32(off)
        offset += 4

    case OpMonitorAvailability:
        // FacilityName
        facName, newOffset, err := readString(data, offset)
        if err != nil {
            return req, err
        }
        req.FacilityName = facName
        offset = newOffset

        // MonitorPeriod (4 bytes)
        if offset+4 > len(data) {
            return req, fmt.Errorf("not enough bytes for monitorPeriod")
        }
        req.MonitorPeriod = binary.BigEndian.Uint32(data[offset : offset+4])
        offset += 4

    case OpCancelBooking:
        // ConfirmationID
        confID, newOffset, err := readString(data, offset)
        if err != nil {
            return req, err
        }
        req.ConfirmationID = confID
        offset = newOffset

    case OpAddParticipant:
        // ConfirmationID
        confID, newOffset, err := readString(data, offset)
        if err != nil {
            return req, err
        }
        req.ConfirmationID = confID
        offset = newOffset

        // ParticipantName
        part, newOffset2, err := readString(data, offset)
        if err != nil {
            return req, err
        }
        req.ParticipantName = part
        offset = newOffset2

    default:
        return req, fmt.Errorf("unknown OpCode %d", req.OpCode)
    }

    return req, nil
}
func MarshalReply(rep ReplyMessage) ([]byte, error) {
    buf := make([]byte, 0, 64)

    // OpCode (1 byte)
    buf = append(buf, rep.OpCode)

    // RequestID (8 bytes)
    tmp := make([]byte, 8)
    binary.BigEndian.PutUint64(tmp, rep.RequestID)
    buf = append(buf, tmp...)

    // Status (4 bytes)
    tmp4 := make([]byte, 4)
    binary.BigEndian.PutUint32(tmp4, uint32(rep.Status))
    buf = append(buf, tmp4...)

    // Data (2-byte length + bytes)
    buf = writeString(buf, rep.Data)

    return buf, nil
}
func UnmarshalReply(data []byte) (ReplyMessage, error) {
    var rep ReplyMessage
    offset := 0

    // OpCode (1 byte)
    if len(data) < 1 {
        return rep, fmt.Errorf("reply data too short for opcode")
    }
    rep.OpCode = data[offset]
    offset++

    // RequestID (8 bytes)
    if offset+8 > len(data) {
        return rep, fmt.Errorf("reply too short for requestID")
    }
    rep.RequestID = binary.BigEndian.Uint64(data[offset : offset+8])
    offset += 8

    // Status (4 bytes)
    if offset+4 > len(data) {
        return rep, fmt.Errorf("reply too short for status")
    }
    rep.Status = int32(binary.BigEndian.Uint32(data[offset : offset+4]))
    offset += 4

    // Data (string)
    str, newOffset, err := readString(data, offset)
    if err != nil {
        return rep, err
    }
    rep.Data = str
    offset = newOffset

    return rep, nil
}

