# mutual-exclusion

Implementing of the Ricart-Agrawala distributed mutual exclusion algorithm using gRPC.

How to start the system:

Starting nodes:
Open **three separate terminals in the client folder** and run:

Terminal 1:
go run client.go

Terminal 2:
go run client.go

Terminal 3:
go run client.go

Each Node will:

- automatically discover other nodes attending
- assign itself a unique ID(0, 1, or 2)
- Create a log file and write or append to pre-existing log file
- Begin requesting access to the critical section

Stopping the System:

Press ctrl+C in each terminal to shutdown

Viewing Logs:
Logs are written to client/log<id>.txt
