package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Iyzyman/distributed-go/common"
)

// Command-line flags for client
var (
	serverAddrFlag = flag.String("serverAddr", "localhost:2222", "Server address in host:port format")
	timeoutFlag    = flag.Int("timeout", 5, "Timeout in seconds for waiting for server replies")
)

// Global client state
type ClientState struct {
	conn        *net.UDPConn
	serverAddr  *net.UDPAddr
	timeout     time.Duration
	nextReqID   uint64
	monitorMode bool
}

func main() {
	flag.Parse()

	// Parse server address
	serverAddr, err := net.ResolveUDPAddr("udp", *serverAddrFlag)
	if err != nil {
		log.Fatalf("Invalid server address %s: %v", *serverAddrFlag, err)
	}

	// Create UDP socket
	conn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Initialize client state
	client := &ClientState{
		conn:        conn,
		serverAddr:  serverAddr,
		timeout:     time.Duration(*timeoutFlag) * time.Second,
		nextReqID:   uint64(rand.Int63()),
		monitorMode: false,
	}

	fmt.Printf("Connected to server at %s\n", serverAddr)
	fmt.Println("Facility Booking System Client")
	fmt.Println("==============================")

	// Start the CLI
	client.runCLI()
}

// runCLI presents a menu and handles user input
func (c *ClientState) runCLI() {
	reader := bufio.NewReader(os.Stdin)

	for {
		if c.monitorMode {
			fmt.Println("\nMonitoring for updates. Press Enter to return to menu.")
			reader.ReadString('\n')
			c.monitorMode = false
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

// getNextRequestID generates a unique request ID
func (c *ClientState) getNextRequestID() uint64 {
	id := c.nextReqID
	c.nextReqID++
	return id
}

// sendRequest sends a request to the server and waits for a reply
func (c *ClientState) sendRequest(req common.RequestMessage) (*common.ReplyMessage, error) {
	// Marshal the request
	data, err := common.MarshalRequest(req)
	if err != nil {
		return nil, fmt.Errorf("error marshalling request: %v", err)
	}

	// Send to server
	_, err = c.conn.Write(data)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %v", err)
	}

	// Set read deadline for timeout
	err = c.conn.SetReadDeadline(time.Now().Add(c.timeout))
	if err != nil {
		return nil, fmt.Errorf("error setting read deadline: %v", err)
	}

	// Wait for reply
	buffer := make([]byte, 2048)
	n, _, err := c.conn.ReadFromUDP(buffer)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return nil, fmt.Errorf("timeout waiting for server reply")
		}
		return nil, fmt.Errorf("error reading reply: %v", err)
	}

	// Unmarshal the reply
	reply, err := common.UnmarshalReply(buffer[:n])
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling reply: %v", err)
	}

	return &reply, nil
}

// readDaysList prompts the user for a list of days
func readDaysList(reader *bufio.Reader) ([]uint8, error) {
	fmt.Print("Enter number of days to check: ")
	numDaysStr, _ := reader.ReadString('\n')
	numDaysStr = strings.TrimSpace(numDaysStr)
	numDays, err := strconv.Atoi(numDaysStr)
	if err != nil || numDays <= 0 {
		return nil, fmt.Errorf("invalid number of days")
	}

	fmt.Println("Enter day indices (0=Monday, 1=Tuesday, ..., 6=Sunday):")
	days := make([]uint8, 0, numDays)
	for i := 0; i < numDays; i++ {
		fmt.Printf("Day %d: ", i+1)
		dayStr, _ := reader.ReadString('\n')
		dayStr = strings.TrimSpace(dayStr)
		day, err := strconv.Atoi(dayStr)
		if err != nil || day < 0 || day > 6 {
			return nil, fmt.Errorf("invalid day index (must be 0-6)")
		}
		days = append(days, uint8(day))
	}
	return days, nil
}

// readBookingTimes prompts the user for booking start/end times
func readBookingTimes(reader *bufio.Reader) (uint8, uint8, uint8, uint8, uint8, uint8, error) {
	fmt.Print("Enter start day (0=Monday..6=Sunday): ")
	startDayStr, _ := reader.ReadString('\n')
	startDay, err := strconv.Atoi(strings.TrimSpace(startDayStr))
	if err != nil || startDay < 0 || startDay > 6 {
		return 0, 0, 0, 0, 0, 0, fmt.Errorf("invalid start day")
	}

	fmt.Print("Enter start hour (0-23): ")
	startHourStr, _ := reader.ReadString('\n')
	startHour, err := strconv.Atoi(strings.TrimSpace(startHourStr))
	if err != nil || startHour < 0 || startHour > 23 {
		return 0, 0, 0, 0, 0, 0, fmt.Errorf("invalid start hour")
	}

	fmt.Print("Enter start minute (0-59): ")
	startMinStr, _ := reader.ReadString('\n')
	startMin, err := strconv.Atoi(strings.TrimSpace(startMinStr))
	if err != nil || startMin < 0 || startMin > 59 {
		return 0, 0, 0, 0, 0, 0, fmt.Errorf("invalid start minute")
	}

	fmt.Print("Enter end day (0=Monday..6=Sunday): ")
	endDayStr, _ := reader.ReadString('\n')
	endDay, err := strconv.Atoi(strings.TrimSpace(endDayStr))
	if err != nil || endDay < 0 || endDay > 6 {
		return 0, 0, 0, 0, 0, 0, fmt.Errorf("invalid end day")
	}

	fmt.Print("Enter end hour (0-23): ")
	endHourStr, _ := reader.ReadString('\n')
	endHour, err := strconv.Atoi(strings.TrimSpace(endHourStr))
	if err != nil || endHour < 0 || endHour > 23 {
		return 0, 0, 0, 0, 0, 0, fmt.Errorf("invalid end hour")
	}

	fmt.Print("Enter end minute (0-59): ")
	endMinStr, _ := reader.ReadString('\n')
	endMin, err := strconv.Atoi(strings.TrimSpace(endMinStr))
	if err != nil || endMin < 0 || endMin > 59 {
		return 0, 0, 0, 0, 0, 0, fmt.Errorf("invalid end minute")
	}

	return uint8(startDay), uint8(startHour), uint8(startMin),
		uint8(endDay), uint8(endHour), uint8(endMin), nil
}

// handleQueryAvailability implements the Query operation
func (c *ClientState) handleQueryAvailability(reader *bufio.Reader) {
	fmt.Print("Enter facility name: ")
	facilityName, _ := reader.ReadString('\n')
	facilityName = strings.TrimSpace(facilityName)

	days, err := readDaysList(reader)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Create request
	req := common.RequestMessage{
		OpCode:       common.OpQueryAvailability,
		RequestID:    c.getNextRequestID(),
		FacilityName: facilityName,
		DaysList:     days,
	}

	// Send request and get reply
	reply, err := c.sendRequest(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Display result
	fmt.Println("\nQuery Result:")
	fmt.Println(reply.Data)
}

// handleBookFacility implements the Book operation
func (c *ClientState) handleBookFacility(reader *bufio.Reader) {
	fmt.Print("Enter facility name: ")
	facilityName, _ := reader.ReadString('\n')
	facilityName = strings.TrimSpace(facilityName)

	startDay, startHour, startMin, endDay, endHour, endMin, err := readBookingTimes(reader)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Create request
	req := common.RequestMessage{
		OpCode:       common.OpBookFacility,
		RequestID:    c.getNextRequestID(),
		FacilityName: facilityName,
		StartDay:     startDay,
		StartHour:    startHour,
		StartMinute:  startMin,
		EndDay:       endDay,
		EndHour:      endHour,
		EndMinute:    endMin,
	}

	// Send request and get reply
	reply, err := c.sendRequest(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Display result
	if reply.Status == 0 {
		fmt.Println("\nBooking successful!")
	} else {
		fmt.Println("\nBooking failed!")
	}
	fmt.Println(reply.Data)
}

// handleChangeBooking implements the Change operation
func (c *ClientState) handleChangeBooking(reader *bufio.Reader) {
	fmt.Print("Enter Confirmation ID: ")
	confirmationID, _ := reader.ReadString('\n')
	confirmationID = strings.TrimSpace(confirmationID)

	startDay, startHour, startMin, endDay, endHour, endMin, err := readBookingTimes(reader)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Create request
	req := common.RequestMessage{
		OpCode:         common.OpChangeBooking,
		RequestID:      c.getNextRequestID(),
		ConfirmationID: confirmationID,
		StartDay:       startDay,
		StartHour:      startHour,
		StartMinute:    startMin,
		EndDay:         endDay,
		EndHour:        endHour,
		EndMinute:      endMin,
	}

	// Send request and get reply
	reply, err := c.sendRequest(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Display result
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
		RequestID:     c.getNextRequestID(),
		FacilityName:  facilityName,
		MonitorPeriod: uint32(duration),
	}

	// Send request and get reply
	reply, err := c.sendRequest(req)
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
	c.monitorMode = true

	// Start a goroutine to listen for callbacks
	go func() {
		buffer := make([]byte, 2048)
		for c.monitorMode {
			// Set a short timeout so we can check if monitoring mode is still active
			c.conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			n, _, err := c.conn.ReadFromUDP(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					// Just a timeout, continue
					continue
				}
				if c.monitorMode {
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

			fmt.Printf("\nCallback received: %s\n", callback.Data)
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
		RequestID:      c.getNextRequestID(),
		ConfirmationID: confirmationID,
	}

	// Send request and get reply
	reply, err := c.sendRequest(req)
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
		RequestID:       c.getNextRequestID(),
		ConfirmationID:  confirmationID,
		ParticipantName: participantName,
	}

	// Send request and get reply
	reply, err := c.sendRequest(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Display result
	if reply.Status == 0 {
		fmt.Println("\nParticipant added successfully!")
	} else {
		fmt.Println("\nFailed to add participant!")
	}
	fmt.Println(reply.Data)
}
