package common

// Operation codes
const (
	OpQueryAvailability   = 1
	OpBookFacility        = 2
	OpChangeBooking       = 3
	OpMonitorAvailability = 4
	OpCancelBooking       = 5
	OpAddParticipant      = 6
)

// RequestMessage holds all possible input fields for any operation.
type RequestMessage struct {
	OpCode    uint8
	RequestID uint64

	// Common fields
	FacilityName string // Used by Query, Book, Monitor, etc.

	// For QueryAvailability
	DaysList []uint8 // e.g., day indices 0..6 for Monday..Sunday

	// For BookFacility
	StartDay    uint8
	StartHour   uint8
	StartMinute uint8
	EndDay      uint8
	EndHour     uint8
	EndMinute   uint8

	// For ChangeBooking / CancelBooking / AddParticipant
	ConfirmationID string
	OffsetMinutes  int32
	// For MonitorAvailability
	MonitorPeriod uint32

	// For AddParticipant
	ParticipantName string
}

// ReplyMessage is returned by the server to the client
type ReplyMessage struct {
	RequestID uint64
	OpCode    uint8  // optional if you want to echo the operation code
	Status    int32  // 0 for success, negative/positive for errors
	Data      string // e.g., booking ID, schedule info, error message, etc.
}
