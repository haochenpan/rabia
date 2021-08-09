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
package serialization

import (
	"bufio"
	"fmt"
	"math/rand"
	"net"
	"reflect"
	"sync"
	"time"
)

func SetupNetwork() (net.Listener, *net.Conn, *net.Conn) {
	listener, err := net.Listen("tcp", ":18080")
	if err != nil {
		panic(err)
	}

	var conn1 *net.Conn
	var conn2 *net.Conn

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		for {
			conn, err := net.Dial("tcp", ":18080")
			if err == nil {
				conn1 = &conn
				wg.Done()
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()
	conn, err := listener.Accept()
	if err != nil {
		panic(err)
	}
	conn2 = &conn
	wg.Wait()

	return listener, conn1, conn2
}

func CloseNetwork(listener net.Listener, conn1, conn2 *net.Conn) {
	_ = (*conn1).Close()
	_ = (*conn2).Close()
	_ = listener.Close()
}

func GetReaderWriter(conn *net.Conn) (*bufio.Reader, *bufio.Writer) {
	var err error
	err = (*conn).(*net.TCPConn).SetWriteBuffer(7000000)
	if err != nil {
		panic("should not happen")
	}
	err = (*conn).(*net.TCPConn).SetReadBuffer(7000000)
	if err != nil {
		panic("should not happen")
	}
	err = (*conn).(*net.TCPConn).SetKeepAlive(true)
	if err != nil {
		panic("should not happen")
	}
	err = (*conn).(*net.TCPConn).SetKeepAlivePeriod(20 * time.Second)
	if err != nil {
		panic("should not happen")
	}
	reader := bufio.NewReaderSize(*conn, 4096*4000)
	writer := bufio.NewWriterSize(*conn, 4096*4000)
	return reader, writer
}

/*
	https://stackoverflow.com/q/6395076
*/
func RandGoBinMsg() *GoBinMsg {
	s := &GoBinMsg{}
	v := reflect.ValueOf(s).Elem()

	for i := 0; i < v.NumField(); i++ {
		v.Field(i).SetInt(rand.Int63())
	}
	return s
}

func RandProtoMsg() *ProtoMsg {
	s := &ProtoMsg{}
	v := reflect.ValueOf(s).Elem()
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).CanSet() { // a Key or a Val field
			v.Field(i).SetInt(rand.Int63())
		}
	}
	return s
}

func RandGoGoMsg() *GoGoMsg {
	s := &GoGoMsg{}
	v := reflect.ValueOf(s).Elem()
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).CanSet() { // a Key or a Val field
			v.Field(i).SetInt(rand.Int63())
		}
	}
	return s
}

func PrintGoBinMsg(msg *GoBinMsg) string {
	return fmt.Sprint(reflect.ValueOf(msg).Elem())
}

func PrintProtoMsg(msg *ProtoMsg) string {
	return msg.String()
}

func PrintGoGoMsg(msg *GoGoMsg) string {
	return msg.String()
}

func PrepareGoBinMsgArray() []GoBinMsg {
	msgArray := make([]GoBinMsg, 1000)
	for i := 0; i < len(msgArray); i++ {
		msgArray[i] = *RandGoBinMsg()
	}
	return msgArray
}

func PrepareProtoMsgArray() []ProtoMsg {
	msgArray := make([]ProtoMsg, 1000)
	for i := 0; i < len(msgArray); i++ {
		msgArray[i] = *RandProtoMsg()
	}
	return msgArray
}

func PrepareGoGoMsgArray() []GoGoMsg {
	msgArray := make([]GoGoMsg, 1000)
	for i := 0; i < len(msgArray); i++ {
		msgArray[i] = *RandGoGoMsg()
	}
	return msgArray
}
