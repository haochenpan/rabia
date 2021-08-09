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
	"bytes"
	"google.golang.org/protobuf/proto"
	"io"
	"sync"
	"testing"
)

/*
	cd ~/go/src/rc3/serialization_test/serialization
	go test -bench=. -test.benchtime=2s
*/

var readBufSize = 4000 * 1024

func TestRandGoBinMsg(t *testing.T) {
	msg := RandGoBinMsg()
	t.Log(PrintGoBinMsg(msg))
}

func TestRandProtoMsg(t *testing.T) {
	msg := RandProtoMsg()
	t.Log(PrintProtoMsg(msg))
}

func TestRandGoGoMsg(t *testing.T) {
	msg := RandGoGoMsg()
	t.Log(PrintGoGoMsg(msg))
}

func TestGoBin_Local_OneMsg(t *testing.T) {
	q1 := RandGoBinMsg()
	t.Log("Prepare to send data:", PrintGoBinMsg(q1))
	buf := new(bytes.Buffer)
	q1.Marshal(buf)

	q2 := &GoBinMsg{}
	if err := q2.Unmarshal(buf); err != nil {
		panic(err)
	}
	t.Log("Received data:", PrintGoBinMsg(q2))
}

func TestGoBin_Network_OneMsg(t *testing.T) {
	listener, conn1, conn2 := SetupNetwork()
	_, conn1Writer := GetReaderWriter(conn1)
	conn2Reader, _ := GetReaderWriter(conn2)

	q1 := RandGoBinMsg()
	t.Log("Prepare to send data:", PrintGoBinMsg(q1))
	q1.Marshal(conn1Writer)
	_ = conn1Writer.Flush()

	q2 := &GoBinMsg{}
	if err := q2.Unmarshal(conn2Reader); err != nil {
		panic(err)
	}
	t.Log("Received data:", PrintGoBinMsg(q2))

	CloseNetwork(listener, conn1, conn2)
}

func TestProto_Local_OneMsg(t *testing.T) {
	q1 := RandProtoMsg()
	t.Log("Prepare to send data:", PrintProtoMsg(q1))
	buf := new(bytes.Buffer)
	data, _ := proto.Marshal(q1)
	buf.Write(data)

	q2 := &ProtoMsg{}
	read := make([]byte, readBufSize)
	n := proto.Size(q2)
	_, _ = io.ReadFull(buf, read[:n])
	if err := proto.Unmarshal(read[:n], q2); err != nil {
		panic(err)
	}
	t.Log("Received data:", PrintProtoMsg(q2))
}

func TestProto_Network_OneMsg(t *testing.T) {
	listener, conn1, conn2 := SetupNetwork()
	_, conn1Writer := GetReaderWriter(conn1)
	conn2Reader, _ := GetReaderWriter(conn2)

	q1 := RandProtoMsg()
	t.Log("Prepare to send data:", PrintProtoMsg(q1))
	data, _ := proto.Marshal(q1)
	_, _ = conn1Writer.Write(data)
	_ = conn1Writer.Flush()

	q2 := &ProtoMsg{}
	read := make([]byte, readBufSize)
	n := proto.Size(q2)
	_, _ = io.ReadFull(conn2Reader, read[:n])
	if err := proto.Unmarshal(read[:n], q2); err != nil {
		panic(err)
	}
	t.Log("Received data:", PrintProtoMsg(q2))

	CloseNetwork(listener, conn1, conn2)
}

func TestGoGo_Local_OneMsg(t *testing.T) {
	q1 := RandGoGoMsg()
	t.Log("Prepare to send data:", PrintGoGoMsg(q1))
	buf := new(bytes.Buffer)
	data, _ := q1.Marshal()
	buf.Write(data)

	q2 := &GoGoMsg{}
	read := make([]byte, readBufSize)
	n := q2.Size()
	_, _ = io.ReadFull(buf, read[:n])
	if err := q2.Unmarshal(read[:n]); err != nil {
		panic(err)
	}
	t.Log("Received data:", PrintGoGoMsg(q2))
}

func TestGoGo_Network_OneMsg(t *testing.T) {
	listener, conn1, conn2 := SetupNetwork()
	_, conn1Writer := GetReaderWriter(conn1)
	conn2Reader, _ := GetReaderWriter(conn2)

	q1 := RandGoGoMsg()
	t.Log("Prepare to send data:", PrintGoGoMsg(q1))
	data, _ := q1.Marshal()
	_, _ = conn1Writer.Write(data)
	_ = conn1Writer.Flush()

	q2 := &GoGoMsg{}
	read := make([]byte, readBufSize)
	n := q2.Size()
	_, _ = io.ReadFull(conn2Reader, read[:n])
	if err := q2.Unmarshal(read[:n]); err != nil {
		panic(err)
	}
	t.Log("Received data:", PrintGoGoMsg(q2))

	CloseNetwork(listener, conn1, conn2)
}

/*
	Note: ProtoMsg and GoGoMsg messages have dynamic sizes
*/

/*
	SetBytes: conn2's bytes received per second
*/
func BenchmarkGoBin_Network_OneDirection(b *testing.B) {
	listener, conn1, conn2 := SetupNetwork()
	_, conn1Writer := GetReaderWriter(conn1)
	conn2Reader, _ := GetReaderWriter(conn2)

	msgArray := PrepareGoBinMsgArray()
	wg := sync.WaitGroup{}
	wg.Add(2)
	b.ResetTimer()

	go func() {
		defer wg.Done()

		for i := 0; i < b.N; i++ {
			msgArray[i%len(msgArray)].Marshal(conn1Writer)
			_ = conn1Writer.Flush()
		}
	}()

	go func() {
		defer wg.Done()
		q2 := &GoBinMsg{}

		for i := 0; i < b.N; i++ {
			if err := q2.Unmarshal(conn2Reader); err != nil {
				b.Error("Could not unmarshal buf: ", err)
			}

			n, _ := q2.BinarySize()
			b.SetBytes(int64(n))
		}
	}()

	wg.Wait()
	CloseNetwork(listener, conn1, conn2)
}

/*
	SetBytes: conn2's bytes received per second
*/
func BenchmarkProto_Network_OneDirection(b *testing.B) {
	listener, conn1, conn2 := SetupNetwork()
	_, conn1Writer := GetReaderWriter(conn1)
	conn2Reader, _ := GetReaderWriter(conn2)

	msgArray := PrepareProtoMsgArray()
	wg := sync.WaitGroup{}
	wg.Add(2)
	b.ResetTimer()

	go func() {
		defer wg.Done()

		for i := 0; i < b.N; i++ {
			data, _ := proto.Marshal(&msgArray[i%len(msgArray)])
			_, _ = conn1Writer.Write(data)
			_ = conn1Writer.Flush()
		}
	}()

	go func() {
		defer wg.Done()
		q2 := &ProtoMsg{}
		read := make([]byte, readBufSize)

		for i := 0; i < b.N; i++ {
			n := proto.Size(&msgArray[i%len(msgArray)])
			_, _ = io.ReadFull(conn2Reader, read[:n])
			if err := proto.Unmarshal(read[:n], q2); err != nil {
				panic(err)
			}

			b.SetBytes(int64(n))
		}
	}()

	wg.Wait()
	CloseNetwork(listener, conn1, conn2)
}

/*
	SetBytes: conn2's bytes received per second
*/
func BenchmarkGoGo_Network_OneDirection(b *testing.B) {
	listener, conn1, conn2 := SetupNetwork()
	_, conn1Writer := GetReaderWriter(conn1)
	conn2Reader, _ := GetReaderWriter(conn2)

	msgArray := PrepareGoGoMsgArray()
	wg := sync.WaitGroup{}
	wg.Add(2)
	b.ResetTimer()

	go func() {
		defer wg.Done()

		for i := 0; i < b.N; i++ {
			data, _ := msgArray[i%len(msgArray)].Marshal()
			_, _ = conn1Writer.Write(data)
			_ = conn1Writer.Flush()
		}
	}()

	go func() {
		defer wg.Done()
		q2 := &GoGoMsg{}
		read := make([]byte, readBufSize)

		for i := 0; i < b.N; i++ {
			n := msgArray[i%len(msgArray)].Size()
			_, _ = io.ReadFull(conn2Reader, read[:n])
			if err := q2.Unmarshal(read[:n]); err != nil {
				panic(err)
			}

			b.SetBytes(int64(n))
		}
	}()

	wg.Wait()
	CloseNetwork(listener, conn1, conn2)
}
