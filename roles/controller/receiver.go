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
package controller

import (
	"bufio"
	"fmt"
	"net"
	. "rabia/internal/config"
	. "rabia/internal/message"
	"rabia/internal/tcp"
	"time"
)

/*
	A benchmarking control command receiver, installed at each server or client.
*/
type Receiver struct {
	Controller   *net.Conn
	ReadWriter   *bufio.ReadWriter
	ServerClient uint32 // 1 == client, 2 == server
	Id           uint32
}

/*
	Initialize a receiver, fields are filled differently based on whether the caller is a server or a client
*/
func ReceiverInit(id uint32, isClient bool) *Receiver {
	c := &Receiver{Id: id}
	if isClient {
		c.ServerClient = 1
	} else {
		c.ServerClient = 2
	}
	return c
}

/*
	Connect to the controller
*/
func (c *Receiver) Connect() {
	var conn net.Conn
	var err error
	for {
		conn, err = net.Dial("tcp", Conf.ControllerAddr)
		if err == nil {
			err = conn.(*net.TCPConn).SetKeepAlive(true)
			if err != nil {
				panic(fmt.Sprint("should not happen", err))
			}
			err = conn.(*net.TCPConn).SetKeepAlivePeriod(20 * time.Second)
			if err != nil {
				panic(fmt.Sprint("should not happen", err))
			}
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	c.Controller = &conn
	reader, writer := tcp.GetReaderWriter(&conn)
	c.ReadWriter = bufio.NewReadWriter(reader, writer)
}

/*
	Send a message to the controller
*/
func (c *Receiver) MsgToController() {
	r := Command{CliId: c.Id, CliSeq: c.ServerClient}
	err := r.MarshalWriteFlush(c.ReadWriter.Writer)
	if err != nil {
		panic(fmt.Sprint("should not happen", err))
	}
}

/*
	Wait the controller to send a message
*/
func (c *Receiver) WaitController() {
	r := Command{}
	readBuf := make([]byte, 20)
	err := r.ReadUnmarshal(c.ReadWriter.Reader, readBuf)
	if err != nil {
		panic(fmt.Sprint("should not happen", err))
	}
}
