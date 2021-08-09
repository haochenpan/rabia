/*
    Copyright 2021 Rabia Research Team and Developers

    Licensed under the Apache License, Version 2.0 (the "License");
    you may not use this file except in compliance with the License.
    You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

    Unless required by applicable law or agreed to in writing, software
    distributed under the License is distributed on an "AS IS" BASIS,
    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    See the License for the specific language governing permissions and
    limitations under the License.
*/
/*
	The main package contains Rabia's entry function, which loads configurations and spawns a Rabia server, or a client,
	or a benchmarking controller based on provided arguments.
*/
package main

import (
	. "rabia/internal/config"
	"rabia/roles/client"
	"rabia/roles/controller"
	"rabia/roles/server"
	"strconv"
	"syscall"
	"time"
)

/*
	The main function first loads various configurations provided through environmental variables, the command line, and
	hard-coded constants in config.go. Then starts a server/client/controller based on the variable Conf.Role.
*/
func main() {
	Conf.LoadConfigs()
	if Conf.Role == "ctrl" {
		controller.RunController()
	} else if Conf.Role == "svr" {
		idx, _ := strconv.Atoi(Conf.Id)
		RunServer(uint32(idx))
	} else if Conf.Role == "cli" {
		idx, _ := strconv.Atoi(Conf.Id)
		RunClient(uint32(idx))
	} else {
		panic("should not happen, error Conf.Role")
	}
}

/*
	Runs a Rabia server
*/
func RunServer(idx uint32) {
	// Initialization and establishing peer connections, see comments inside functions
	svr := server.ServerInit(idx, Conf.SvrIp+":"+Conf.ProxyPort, Conf.SvrIp+":"+Conf.NetworkPort)
	svr.Prologue()

	// Initiate a command receiver that listens to the benchmark controller
	receiver := controller.ReceiverInit(idx, false)
	receiver.Connect()
	receiver.MsgToController() // Notify the controller this server is ready to benchmark

	// The receiver does not run on the main server thread,
	// since it interrupts the main server thread (to shutdown the server) when cluster benchmarking is done
	go func() {
		receiver.WaitController()
		time.Sleep(2 * time.Second) // Wait inter-server messaging to be completed
		_ = syscall.Kill(syscall.Getpid(), syscall.SIGINT)
	}()

	// The main task and the exit actions
	svr.ServerMain()
	svr.Epilogue()

	// Notify the controller that this server has exited
	receiver.MsgToController()
}

/*
	Runs a Rabia client
*/
func RunClient(idx uint32) {
	// Initialization and proxy connection, see comments inside functions
	cli := client.ClientInit(idx, Conf.ProxyAddr)
	cli.Prologue()

	// Initiate a command receiver that listens to the benchmark controller
	receiver := controller.ReceiverInit(uint32(idx), true)
	receiver.Connect()
	receiver.MsgToController()
	receiver.WaitController()

	// The client's main task and the exit actions
	if Conf.ClosedLoop {
		cli.CloseLoopClient()
	} else {
		cli.OpenLoopClient()
	}
	cli.Epilogue()

	// Notify the controller that this client has exited
	receiver.MsgToController()
}
