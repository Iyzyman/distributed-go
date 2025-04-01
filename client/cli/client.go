package cli

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/Iyzyman/distributed-go/client/utils"
	"github.com/Iyzyman/distributed-go/common"
)

// ClientState represents the global client state
type ClientState struct {
	Conn        *net.UDPConn
	ServerAddr  *net.UDPAddr
	Timeout     time.Duration
	NextReqID   uint64
	MonitorMode bool
	PacketDemo  bool
}

// RunCLI presents a menu and handles user input
func (c *ClientState) RunCLI() {
	reader := bufio.NewReader(os.Stdin)

	for {
		if c.MonitorMode {
			fmt.Println("\nMonitoring for updates. Press Enter to return to menu.")
			reader.ReadString('\n')
			c.MonitorMode = false
			continue
		}

		fmt.Println("\nAvailable commands:")
		fmt.Println("1. query - Query facility availability")
		fmt.Println("2. book - Book a facility")
		fmt.Println("3. change - Change an existing booking")
		fmt.Println("4. monitor - Monitor facility availability")
		fmt.Println("5. cancel - Cancel a booking")
		fmt.Println("6. add-participant - Add participant to a booking")
		fmt.Println("7. exit - Exit the client")
		fmt.Print("\nEnter command: ")

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		switch input {
		case "1", "query":
			c.handleQueryAvailability(reader)
		case "2", "book":
			c.handleBookFacility(reader)
		case "3", "change":
			c.handleChangeBooking(reader)
		case "4", "monitor":
			c.handleMonitorAvailability(reader)
		case "5", "cancel":
			c.handleCancelBooking(reader)
		case "6", "add-participant":
			c.handleAddParticipant(reader)
		case "7", "exit":
			fmt.Println("Exiting client.")
			return
		default:
			fmt.Println("Unknown command. Please try again.")
		}
	}
}

// GetNextRequestID generates a unique request ID
func (c *ClientState) GetNextRequestID() uint64 {
	id := c.NextReqID
	c.NextReqID++
	return id
}

// SendRequest sends a request to the server and waits for a reply
func (c *ClientState) SendRequest(req common.RequestMessage) (*common.ReplyMessage, error) {
    data, err := common.MarshalRequest(req)
    if err != nil {
        return nil, fmt.Errorf("error marshalling: %w", err)
    }
    maxRetries := 4
    for attempt := 0; attempt < maxRetries; attempt++ {
        // Send the request
        _, err = c.Conn.Write(data)
        if err != nil {
            return nil, fmt.Errorf("error sending request: %w", err)
        }
        
        // Set deadline
        c.Conn.SetReadDeadline(time.Now().Add(c.Timeout))

        // Wait for reply
        buffer := make([]byte, 2048)
        n, _, err := c.Conn.ReadFromUDP(buffer)
        if err == nil {
            // Check if we're simulating a packet loss after receiving a valid reply
			value := rand.Float32()
			
            if c.PacketDemo && value < 0.5 {
                fmt.Printf("Simulating lost reply on attempt %d (packetDemo=true)\n", attempt+1)
                // Pretend no data was received => force a timeout-like scenario, so the loop retries.
                fmt.Printf("Timeout on attempt %d, retrying...\n", attempt+1)
                continue
            }

            // If no simulated packet loss, proceed with normal unmarshal
            reply, umErr := common.UnmarshalReply(buffer[:n])
            if umErr != nil {
                return nil, fmt.Errorf("error unmarshalling reply: %w", umErr)
            }
            return &reply, nil
        }

        if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
            // timed out, go for next attempt
            fmt.Printf("Timeout on attempt %d, retrying...\n", attempt+1)
            continue
        }
        
        // If it's some other error, break immediately
        return nil, fmt.Errorf("error reading reply: %w", err)
    }
    // If we exhaust all retries
    return nil, fmt.Errorf("no reply after %d attempts", maxRetries)
}

// handleQueryAvailability implements the Query operation
func (c *ClientState) handleQueryAvailability(reader *bufio.Reader) {
	fmt.Print("Enter facility name: ")
	facilityName, _ := reader.ReadString('\n')
	facilityName = strings.TrimSpace(facilityName)

	days, err := utils.ReadDaysList(reader)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Create request
	req := common.RequestMessage{
		OpCode:       common.OpQueryAvailability,
		RequestID:    c.GetNextRequestID(),
		FacilityName: facilityName,
		DaysList:     days,
	}

	// Send request and get reply
	reply, err := c.SendRequest(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Display result
	fmt.Println("\nQuery Result:")
	if reply.Status == 0 {
		fmt.Println(reply.Data)
	} else {
		fmt.Printf("Error: %s\n", reply.Data)
	}
}

// handleBookFacility implements the Book operation
func (c *ClientState) handleBookFacility(reader *bufio.Reader) {
	fmt.Print("Enter facility name: ")
	facilityName, _ := reader.ReadString('\n')
	facilityName = strings.TrimSpace(facilityName)

	startDay, startHour, startMin, endDay, endHour, endMin, err := utils.ReadBookingTimes(reader)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Create request
	req := common.RequestMessage{
		OpCode:       common.OpBookFacility,
		RequestID:    c.GetNextRequestID(),
		FacilityName: facilityName,
		StartDay:     startDay,
		StartHour:    startHour,
		StartMinute:  startMin,
		EndDay:       endDay,
		EndHour:      endHour,
		EndMinute:    endMin,
	}

	// Send request and get reply
	reply, err := c.SendRequest(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Display result
	if reply.Status == 0 {
		fmt.Println("\nBooking successful!")
		fmt.Println(reply.Data)
	} else {
		fmt.Println("\nBooking failed!")
		fmt.Printf("Error: %s\n", reply.Data)
	}
}

// handleChangeBooking implements the Change operation using an offset.
func (c *ClientState) handleChangeBooking(reader *bufio.Reader) {
    // Prompt for the booking confirmation ID.
    fmt.Print("Enter Confirmation ID: ")
    confirmationID, _ := reader.ReadString('\n')
    confirmationID = strings.TrimSpace(confirmationID)

    // Prompt for the offset (in minutes).
    fmt.Print("Enter offset in minutes (positive to advance, negative to postpone): ")
    offsetStr, _ := reader.ReadString('\n')
    offsetStr = strings.TrimSpace(offsetStr)
    offset, err := strconv.Atoi(offsetStr)
    if err != nil {
        fmt.Printf("Error parsing offset: %v\n", err)
        return
    }

    // Create the request with the offset.
    req := common.RequestMessage{
        OpCode:         common.OpChangeBooking,
        RequestID:      c.GetNextRequestID(),
        ConfirmationID: confirmationID,
        OffsetMinutes:  int32(offset),
    }

    // Send request and get reply.
    reply, err := c.SendRequest(req)
    if err != nil {
        fmt.Printf("Error sending request: %v\n", err)
        return
    }

    // Display result.
    if reply.Status == 0 {
        fmt.Println("\nBooking changed successfully!")
    } else {
        fmt.Println("\nFailed to change booking!")
    }
    fmt.Println(reply.Data)
}

// handleMonitorAvailability implements the Monitor operation
func (c *ClientState) handleMonitorAvailability(reader *bufio.Reader) {
	fmt.Print("Enter facility name: ")
	facilityName, _ := reader.ReadString('\n')
	facilityName = strings.TrimSpace(facilityName)

	fmt.Print("Enter duration in seconds: ")
	durationStr, _ := reader.ReadString('\n')
	duration, err := strconv.Atoi(strings.TrimSpace(durationStr))
	if err != nil || duration <= 0 {
		fmt.Println("Error: Invalid duration")
		return
	}

	// Create request
	req := common.RequestMessage{
		OpCode:        common.OpMonitorAvailability,
		RequestID:     c.GetNextRequestID(),
		FacilityName:  facilityName,
		MonitorPeriod: uint32(duration),
	}

	// Send request and get reply
	reply, err := c.SendRequest(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if reply.Status != 0 {
		fmt.Println("\nFailed to start monitoring!")
		fmt.Println(reply.Data)
		return
	}

	fmt.Println("\nMonitoring started successfully!")
	fmt.Println(reply.Data)
	fmt.Println("\nWaiting for updates (press Enter to stop)...")

	// Set up for monitoring callbacks
	c.MonitorMode = true

	// Start a goroutine to listen for callbacks
	go func() {
		buffer := make([]byte, 2048)
		for c.MonitorMode {
			// Set a short timeout so we can check if monitoring mode is still active
			c.Conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			n, _, err := c.Conn.ReadFromUDP(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					// Just a timeout, continue
					continue
				}
				if c.MonitorMode {
					fmt.Printf("Error reading callback: %v\n", err)
				}
				return
			}

			// Process the callback
			callback, err := common.UnmarshalReply(buffer[:n])
			if err != nil {
				fmt.Printf("Error unmarshalling callback: %v\n", err)
				continue
			}

			// Only print if it's a monitoring callback
			if strings.Contains(callback.Data, "Facility=") {
				fmt.Printf("\n%s\n", callback.Data)
			}
		}
	}()
}

// handleCancelBooking implements the Cancel operation
func (c *ClientState) handleCancelBooking(reader *bufio.Reader) {
	fmt.Print("Enter Confirmation ID: ")
	confirmationID, _ := reader.ReadString('\n')
	confirmationID = strings.TrimSpace(confirmationID)

	// Create request
	req := common.RequestMessage{
		OpCode:         common.OpCancelBooking,
		RequestID:      c.GetNextRequestID(),
		ConfirmationID: confirmationID,
	}

	// Send request and get reply
	reply, err := c.SendRequest(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Display result
	if reply.Status == 0 {
		fmt.Println("\nBooking canceled successfully!")
	} else {
		fmt.Println("\nFailed to cancel booking!")
	}
	fmt.Println(reply.Data)
}

// handleAddParticipant implements the AddParticipant operation
func (c *ClientState) handleAddParticipant(reader *bufio.Reader) {
	fmt.Print("Enter Booking Confirmation ID: ")
	confirmationID, _ := reader.ReadString('\n')
	confirmationID = strings.TrimSpace(confirmationID)

	fmt.Print("Enter Participant Name: ")
	participantName, _ := reader.ReadString('\n')
	participantName = strings.TrimSpace(participantName)

	// Create request
	req := common.RequestMessage{
		OpCode:          common.OpAddParticipant,
		RequestID:       c.GetNextRequestID(),
		ConfirmationID:  confirmationID,
		ParticipantName: participantName,
	}

	// Send request and get reply
	reply, err := c.SendRequest(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Display result
	if reply.Status == 0 {
		fmt.Println("\nParticipant added successfully!")
		fmt.Println(reply.Data)
	} else {
		fmt.Println("\nFailed to add participant!")
		fmt.Printf("Error: %s\n", reply.Data)
	}
}
