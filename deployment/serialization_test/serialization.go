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
package main

import (
	"bufio"
	"flag"
	"fmt"
	"google.golang.org/protobuf/proto"
	"io"
	"math/rand"
	"net"
	. "rc3/deployment/serialization_test/serialization"
	"time"
)

var idx = flag.Int("idx", 0, "0 or 1")
var iterations = 10000000 / 32
var s0Addr = "10.142.15.225:17070"
var readBufSize = 4000 * 1024
var seed = 715

func main() {
	flag.Parse()
	rand.Seed(int64(seed))
	//fmt.Println("runtime.NumCPU()", runtime.NumCPU())
	msgArray1 := PrepareGoBinMsgArray()
	n, _ := msgArray1[0].BinarySize()
	msgArray2 := PrepareProtoMsgArray()
	msgArray3 := PrepareGoGoMsgArray()
	fmt.Println("msgSizes", n, proto.Size(&msgArray2[0]), msgArray3[0].Size())

	switch *idx {
	case 0:
		listener, conn := SetupNetworkS0()
		_, conn1Writer := GetReaderWriter(conn)

		ProtoS0(msgArray2, conn1Writer)
		GoGoS0(msgArray3, conn1Writer)
		GoBinS0(msgArray1, conn1Writer)

		CloseNetworkS0(listener, conn)

	case 1:
		conn := SetupNetworkS1()
		conn2Reader, _ := GetReaderWriter(conn)

		fmt.Println("Protobuf", ProtoS1(msgArray2, conn2Reader), "ns/op")
		fmt.Println("GoGo", GoGoS1(msgArray3, conn2Reader), "ns/op")
		fmt.Println("GoBin", GoBinS1(conn2Reader), "ns/op")

		CloseNetworkS1(conn)

	default:
		panic("")
	}

}

func SetupNetworkS0() (net.Listener, *net.Conn) {
	listener, err := net.Listen("tcp", s0Addr)
	if err != nil {
		panic(err)
	}

	conn, err := listener.Accept()
	if err != nil {
		panic(err)
	}
	return listener, &conn
}

func SetupNetworkS1() *net.Conn {
	for {
		conn, err := net.Dial("tcp", s0Addr)
		if err == nil {
			return &conn
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func CloseNetworkS0(listener net.Listener, conn *net.Conn) {
	_ = (*conn).Close()
	_ = listener.Close()
}

func CloseNetworkS1(conn *net.Conn) {
	_ = (*conn).Close()
}

func GoBinS0(msgArray []GoBinMsg, conn1Writer *bufio.Writer) {
	for i := 0; i < iterations; i++ {
		msgArray[i%len(msgArray)].Marshal(conn1Writer)
		_ = conn1Writer.Flush()
	}
}

func GoBinS1(conn2Reader *bufio.Reader) int64 {
	q2 := &GoBinMsg{}
	time1 := time.Now()
	for i := 0; i < iterations; i++ {
		if err := q2.Unmarshal(conn2Reader); err != nil {
			panic(err)
		}
	}
	timeDiff := time.Now().Sub(time1).Nanoseconds()
	return timeDiff / int64(iterations)

}

func ProtoS0(msgArray []ProtoMsg, conn1Writer *bufio.Writer) {
	for i := 0; i < iterations; i++ {
		data, _ := proto.Marshal(&msgArray[i%len(msgArray)])
		_, _ = conn1Writer.Write(data)
		_ = conn1Writer.Flush()
	}
}

func ProtoS1(msgArray []ProtoMsg, conn2Reader *bufio.Reader) int64 {
	q2 := &ProtoMsg{}
	read := make([]byte, readBufSize)
	time1 := time.Now()
	for i := 0; i < iterations; i++ {
		n := proto.Size(&msgArray[i%len(msgArray)])
		_, _ = io.ReadFull(conn2Reader, read[:n])
		if err := proto.Unmarshal(read[:n], q2); err != nil {
			panic(err)
		}

	}
	timeDiff := time.Now().Sub(time1).Nanoseconds()
	return timeDiff / int64(iterations)
}

func GoGoS0(msgArray []GoGoMsg, conn1Writer *bufio.Writer) {
	for i := 0; i < iterations; i++ {
		data, _ := msgArray[i%len(msgArray)].Marshal()
		_, _ = conn1Writer.Write(data)
		_ = conn1Writer.Flush()
	}
}

func GoGoS1(msgArray []GoGoMsg, conn2Reader *bufio.Reader) int64 {
	q2 := &GoGoMsg{}
	read := make([]byte, readBufSize)
	time1 := time.Now()
	for i := 0; i < iterations; i++ {
		n := msgArray[i%len(msgArray)].Size()
		_, _ = io.ReadFull(conn2Reader, read[:n])
		if err := q2.Unmarshal(read[:n]); err != nil {
			panic(err)
		}

	}
	timeDiff := time.Now().Sub(time1).Nanoseconds()
	return timeDiff / int64(iterations)
}
