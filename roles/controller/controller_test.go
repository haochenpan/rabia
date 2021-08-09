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
	"fmt"
	"os"
	"path"
	"rabia/internal/config"
	"sync"
	"testing"
	"time"
)

func TestController(t *testing.T) {
	config.Conf.ControllerAddr = ":8070"
	config.Conf.ProjectFolder = "/Users/roger/go/src/rc3"
	err := os.MkdirAll(path.Join(config.Conf.ProjectFolder, "logs"), os.ModePerm)
	if err != nil {
		panic("should not happen")
	}
	config.Conf.NServers = 3
	config.Conf.NFaulty = 1
	config.Conf.NClients = 5
	config.Conf.NConcurrency = 1
	config.Conf.NClientRequests = 1
	config.Conf.ClientThinkTime = 0
	config.Conf.ClientBatchSize = 1
	config.Conf.ProxyBatchSize = 1
	config.Conf.NetworkBatchSize = 1
	config.Conf.NetworkBatchTimeout = -1
	config.Conf.CalcConstants()
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		RunController()
		wg.Done()
	}()
	time.Sleep(1 * time.Second)
	s1 := ReceiverInit(0, false)
	s2 := ReceiverInit(1, false)
	s3 := ReceiverInit(2, false)
	c1 := ReceiverInit(0, true)
	c2 := ReceiverInit(1, true)
	c3 := ReceiverInit(2, true)
	c4 := ReceiverInit(3, true)
	c5 := ReceiverInit(4, true)
	servers := []*Receiver{s1, s2, s3}
	clients := []*Receiver{c1, c2, c3, c4, c5}
	fmt.Println("here")

	for _, r := range clients {
		r.Connect()
		r.MsgToController()
	}
	for _, r := range servers {
		r.Connect()
		r.MsgToController()
	}

	for _, r := range clients {
		r.WaitController()
	}

	time.Sleep(1 * time.Second)

	for _, r := range clients {
		r.MsgToController()
	}

	for _, r := range servers {
		r.WaitController()
		// set the flag
	}

	for _, r := range servers {
		r.MsgToController()
	}
	wg.Wait()
}
