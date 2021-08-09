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
	The controller package defines the struct and functions of the benchmark controller. The controller (1) informs all
	clients to start submitting requests at around the same time. (2) When all clients have done sending or the
	specified execution time is up, they signal the controller, informing all servers to exit. (1) attempts to maximize
	the read and write pressure against Rabia, and purpose (2) helps the shell scripts of Rabia to determine when all
	servers and clients exit so that the scripts can start to gather logs.

	Note:

	1. When Rabia runs in a production environment (i.e., when we are not doing benchmarking), there is no need to have
	a benchmark controller. We need to modify some "superficial" code to remove dependencies on the controller.

	2. The caller shell script waits for the controller instead of sending the controller process to the background as
	it does to the servers and clients. So after servers and clients normally exit, the controller exits, and then the
	script knows it is time to collect logs from the cluster of machines.

	3. The controller communicates with a Receiver object at each server and client to coordinate necessary steps in
	benchmarking.

	The control flow for now:
		servers： done preparing, connect to the controller
		clients： done preparing, connect to the controller
		controller: connected to all servers
		controller: connected to all clients
		controller: send a message to each client to start benchmarking
		controller: receive a message from each client
		controller: send a message to each server to inform the end of benchmarking
		servers: wait for 3 seconds (?), then send a message to the controller
		controller: receive a message from each client, exit

		maybe add a PrepareToStop flag in the future:
		... (the same as above)
		controller: send a message to each server to inform the end of benchmarking
		servers: with a PrepareToStop flag, clearing out queues, send a message to the controller
		controller: receive a message from each client, exit

*/
package controller

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"path"
	. "rabia/internal/config"
	. "rabia/internal/message"
	"rabia/internal/tcp"
	"time"
)

/*
	A Rabia benchmarking controller
*/
type Controller struct {
	Listener net.Listener // connects to all servers and clients
	Servers  []*net.Conn  // server connections
	Clients  []*net.Conn  // client connections

	ServerReadWriters []*bufio.ReadWriter // server TCP readers and writers
	ClientReadWriters []*bufio.ReadWriter // client TCP readers and writers
}

/*
	Controller's main function
*/
func RunController() {
	c := controllerInit()
	c.connect()
	fmt.Println("all servers and clients are connected")
	c.MsgToClients() // start the clients
	printArt()
	c.waitClientReplies() // clients report they are stopped
	fmt.Println("received from all clients")

	c.MsgToServers() // controller stops all servers
	fmt.Println("sent to all servers")
	c.waitServerReplies() // all servers are stopped
	fmt.Println("received from all servers")
}

/*
	Initialize the controller
*/
func controllerInit() *Controller {
	listener, err := net.Listen("tcp", Conf.ControllerAddr)
	if err != nil {
		panic(fmt.Sprint("should not happen", err))
	}

	return &Controller{
		Listener: listener,

		Servers:           make([]*net.Conn, Conf.NServers),
		Clients:           make([]*net.Conn, Conf.NClients),
		ServerReadWriters: make([]*bufio.ReadWriter, Conf.NServers),
		ClientReadWriters: make([]*bufio.ReadWriter, Conf.NClients),
	}
	/*
		Note: entries of the four arrays above are not initialized for now
	*/
}

/*
	Connect to other clients and servers
*/
func (c *Controller) connect() {
	for i := 0; i < Conf.NServers+Conf.NClients; i++ {
		conn, err := c.Listener.Accept()
		if err != nil {
			panic(fmt.Sprint("should not happen", err))
		}
		err = conn.(*net.TCPConn).SetKeepAlive(true)
		if err != nil {
			panic(fmt.Sprint("should not happen", err))
		}
		err = conn.(*net.TCPConn).SetKeepAlivePeriod(20 * time.Second)
		if err != nil {
			panic(fmt.Sprint("should not happen", err))
		}
		reader, writer := tcp.GetReaderWriter(&conn)
		r := Command{} // if CliSeq == 1: client, if CliSeq == 2: server
		readBuf := make([]byte, 20)
		err = r.ReadUnmarshal(reader, readBuf)
		if err != nil {
			panic(fmt.Sprint("should not happen", err))
		}
		if r.CliSeq == uint32(1) {
			if c.Clients[r.CliId] != nil {
				panic(fmt.Sprint("should not happen", err))
			}
			c.Clients[r.CliId] = &conn
			c.ClientReadWriters[r.CliId] = bufio.NewReadWriter(reader, writer)
			//fmt.Println("controller: connected to a client", r.CliId)
		} else if r.CliSeq == uint32(2) {
			if c.Servers[r.CliId] != nil {
				panic(fmt.Sprint("should not happen", err))
			}
			c.Servers[r.CliId] = &conn
			c.ServerReadWriters[r.CliId] = bufio.NewReadWriter(reader, writer)
			//fmt.Println("controller: connected to a server", r.CliId)
		} else {
			panic(fmt.Sprint("should not happen", err))
		}
	}
}

/*
	Print some ASCII art
*/
func printArt() {
	asciiPath := path.Join(Conf.ProjectFolder, "roles/controller/go")
	content, err := ioutil.ReadFile(asciiPath)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(content))
}

/*
	Send a message to all clients
*/
func (c *Controller) MsgToClients() { // start clients
	for _, rw := range c.ClientReadWriters {
		r := &Command{}
		err := r.MarshalWriteFlush(rw.Writer)
		if err != nil {
			panic(fmt.Sprint("should not happen", err))
		}
	}
}

/*
	Wait each client to reply
*/
func (c *Controller) waitClientReplies() { // clients stopped
	for _, rw := range c.ClientReadWriters {
		r := Command{}
		readBuf := make([]byte, 20)
		err := r.ReadUnmarshal(rw.Reader, readBuf)
		if err != nil {
			panic(fmt.Sprint("should not happen", err))
		}
		if r.CliSeq != 1 {
			panic(fmt.Sprint("should not happen", err))
		}
	}
}

/*
	Send a message to all servers
*/
func (c *Controller) MsgToServers() { // stop servers
	for _, rw := range c.ServerReadWriters {
		r := Command{}
		err := r.MarshalWriteFlush(rw.Writer)
		if err != nil {
			panic(fmt.Sprint("should not happen", err))
		}
	}
}

/*
	Wait each server to reply
*/
func (c *Controller) waitServerReplies() { // server stopped
	for _, rw := range c.ServerReadWriters {
		r := Command{}
		readBuf := make([]byte, 20)
		err := r.ReadUnmarshal(rw.Reader, readBuf)
		if err != nil {
			panic(fmt.Sprint("should not happen", err))
		}
		if r.CliSeq != 2 {
			panic(fmt.Sprint("should not happen", err))
		}
	}
}
