package utils

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

// ReadDaysList prompts the user for a list of days
func ReadDaysList(reader *bufio.Reader) ([]uint8, error) {
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

// ReadBookingTimes prompts the user for booking start/end times
func ReadBookingTimes(reader *bufio.Reader) (uint8, uint8, uint8, uint8, uint8, uint8, error) {
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
