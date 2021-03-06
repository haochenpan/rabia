# How to read Shell, Go, and Python code of Rabia

### TL;DR

The **Shell** programs coordinate the benchmarking process by:

1. receive runtime configurations from the user and start one or more Rabia servers/clients or the benchmark
   controllers;
2. collect log files from the cluster when Go programs are exited and call the Python log analysis script.

The **Go** program picks up environmental variables set by the Shell programs and spawns a server/client/controller
based on the provided configurations. Go programs coordinate with each other to complete benchmarking. Some Go routines
generate logs in execution or before exiting.

When all Go programs are exited, the **Python** program analyzes the log files to provide benchmark statistics on 
excel-friendly screen output.

[comment]: <> (### How to read the code &#40;the Shell part&#41;)

### How to read the code (the Go part)

First, read the relevant sections of our paper to obtain a basic understanding of Rabia. Then, read the section below 
and the [Package-level comments](docs/package-level-comments.md) of each Go package. Finally, explore the codebase in an
order you wish (also, a suggested order is given below).


##### The designs explained (short)

Each Rabia instance/program first needs to load environmental variables and hard-coded configurations to the global `Conf` object (see `internal/config`). 
Then, the program becomes one of the three roles: a server, a client, or a controller. See comments in `main.go` for more details.

A **server** has three layers: the proxy/application layer, the peer-communicating network layer, and the consensus
instance layer. A **client** can either be an open-loop one or a closed-loop one based on some provided configurations
in `Conf`. A benchmark **controller** coordinates the benchmarking process (i.e., when should clients start submitting
requests, when servers should exit...) by communicating with a `Receiver` at each server/client. Note that there's no 
benchmark controller in a realistic environment (i.e., not doing benchmarking). See code and detailed comments in
the `roles` folder.

Server and clients listen to OS signals like `SIGINT` and `SIGTERM` in case that the user wants to shun them down (when the program errs, see `internal/system`).
Each client, proxy layer, and network layer has two or more Go routines in charge of sending and receiving messages (see `internal/io`), respectively. 
The server has a `Ledger` object, which stores the server's current view of consensus slots (see `internal/ledger`) and is read by the proxy layer and read and written by the consensus layer. 
Each of client, server, and a server's three layers could have one or more loggers, which log JSON messages to log files for future analysis or verfication or human-friendly messages to the`stdout` for visual inspection (see `internal/logger`). 
A server's consensus layer contains one or more consensus instances, and each consensus instance has a pending request queue (see `internal/queue`). 
Finally, servers and clients communicate through message types defined in `internal/msg/msg.proto` and client requests are random strings generated by `internal/rstring/rstring.go`.

##### Suggested codebase reading sequence

Only relatively important files/package are listed below. View files in this order, read comments at the file header and near code regions. 
To save your time and make the process less boring, skip the functions/files that **YOU THINK** are not important.
Suppose you are researching a particular part of Rabia, for example, how a Rabia client sends and receives messages. In that case, you may start from `roles/client` and work on relevant `internal` packages later on.

- main.go
- internal/msg/msg.proto (important: defines the messaging formats)
- internal/config
- internal/ledger
- internal/queue
- internal/tcp
- roles/server/layers/proxy
- roles/server/layers/network
- roles/server/layers/consensus/executor.go (important: contains the core algorithm)
- roles/client
