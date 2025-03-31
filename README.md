# Distributed Facility Booking System

This is a distributed facility booking system implemented using UDP for client-server communication. The system uses custom message formats for marshalling and unmarshalling data.

## Project Structure

- `/client`: Client implementation
- `/common`: Shared code for message formats and marshalling/unmarshalling
- `/server`: Server implementation

## Running the Server

Start the server with the following command:

```bash
cd server
go run .
```

## Running the Client

Start the client with the following command:

```bash
cd client
go run .
```

## Testing the System

### Testing Different Operations

1. **Query Availability**:
   - Start the client and select option 1 (query)
   - Enter a facility name (e.g., "RoomA")
   - Enter the number of days to check and the day indices (0=Monday, 1=Tuesday, etc.)

2. **Book Facility**:
   - Start the client and select option 2 (book)
   - Enter a facility name (e.g., "RoomA")
   - Enter the start and end times for the booking
   - Note the confirmation ID returned by the server

3. **Change Booking**:
   - Start the client and select option 3 (change)
   - Enter the confirmation ID from a previous booking
   - Enter the new start and end times for the booking

4. **Monitor Availability**:
   - Start the client and select option 4 (monitor)
   - Enter a facility name (e.g., "RoomA")
   - Enter the duration in seconds to monitor
   - In another terminal, start another client and make changes to the facility (book, change, cancel)
   - Observe the callbacks received by the monitoring client

5. **Cancel Booking (Idempotent)**:
   - Start the client and select option 5 (cancel)
   - Enter the confirmation ID from a previous booking
   - Try canceling the same booking again to observe idempotent behavior

6. **Add Participant (Non-Idempotent)**:
   - Start the client and select option 6 (add-participant)
   - Enter the confirmation ID from a previous booking
   - Enter a participant name
   - Try adding the same participant again to observe non-idempotent behavior

### Testing Invocation Semantics

1. **At-Least-Once Semantics**:
   - Start the server with `--semantics=at-least-once`
   - Use the client to perform non-idempotent operations (e.g., add participant)
   - Observation: Repeated calls with the same request ID can cause duplicate effects

2. **At-Most-Once Semantics**:
   - Start the server with `--semantics=at-most-once`
   - Use the client to perform non-idempotent operations (e.g., add participant)
   - Observation: Repeated calls with the same request ID do not cause duplicate effects

## Example Test Scenario

1. Start the server:
   ```bash
   cd server
   go run . --semantics=at-least-once
   ```

2. In another terminal, start a client to monitor a facility:
   ```bash
   cd client
   go run .
   # Select option 4 (monitor)
   # Enter "RoomA" as the facility name
   # Enter 300 seconds as the duration
   ```

3. In a third terminal, start another client to make changes:
   ```bash
   cd client
   go run .
   # Select option 2 (book)
   # Book a time slot in "RoomA"
   # Note the confirmation ID
   
   # Select option 6 (add-participant)
   # Add a participant to the booking
   
   # Select option 3 (change)
   # Change the booking time
   
   # Select option 5 (cancel)
   # Cancel the booking
   ```

4. Observe the callbacks received by the monitoring client in the second terminal.

5. Restart the server with at-most-once semantics and repeat the test to observe the difference in behavior.
