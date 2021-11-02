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
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

/*
	1. Package Description

	The config package defines the Config object that stores most (if not all) parameters needed by a Rabia server, a
	client, or a controller.

	Nearly all other packages access a global Config object named Conf to retrieve OS environmental variables,
	command-line arguments, and hard-coded or calculated parameters.
*/

/*
	Conf is the global object that stores this server's/client's/controller's configurations. It needs to be global
	because functions in other go files of Rabia often needs to use it
*/
var Conf Config

type Config struct {
	/*
		Sec 1. load these fields from system environment variables (see the loadEnvVars1 function)
	*/
	// For all roles, the following 5 fields should be filled
	Role           string // ctrl | svr | cli
	Id             string // 0 | 1 | 2 | ...
	ControllerAddr string // controller's ip:port
	ProjectFolder  string // the project's folder
	LogLevel       string // debug | info | warn

	// If Role == svr, the following 4 fields should be filled
	SvrIp       string   // server ip
	ProxyPort   string   // proxy port (connect by clients)
	NetworkPort string   // network port (connect by all servers)
	Peers       []string // the array of all servers' SvrIp:NetworkPort
	/*
		Peers: an array of lister address, each server uses it to contact every other server to establish TCP connections.
	*/

	// If Role == cli, the following field should be filled
	ProxyAddr string // the client proxy's SvrIp:ProxyPort

	// For all roles, the following fields should be filled
	ClosedLoop bool // whether clients are closed-loop clients

	NServers            int           // the num. of server instances
	NFaulty             int           // the num. of faulty servers (< 1/2 NServers)
	NClients            int           // the num. of clients
	NConcurrency        int           // the num. of concurrent consensus instances (= concurrency >= 1)
	NClientRequests     int           // the num. of requests PER client, open-loop only
	ClientThinkTime     int           // the think time between sending two requests (ms)
	ClientBatchSize     int           // the num. of DB operations in a client's request
	ProxyBatchSize      int           // the num. of client requests in a consensus object
	ProxyBatchTimeout   time.Duration // the max. time between submitting requests (ns, nanosecond)
	NetworkBatchSize    int           // reserved
	NetworkBatchTimeout time.Duration // reserved (ms, Millisecond)

	/*
		Sec 2. load these fields from the LoadConst function, they are not assigned from environment variables because
		there is no need to override the original value below in most cases
	*/
	NMinusF       int // the "constant" n - f
	Majority      int
	MajorityPlusF int
	FaultyPlusOne int

	LenLedger     uint32 // the length of a ledger ring buffer
	LenBlockArray int    // the length of each ledger block's array
	LenChannel    int    // the length of buffer channels (excepted the channel Q in a Block Block)
	LenPQueue     int    // the length of each priority queue's initial capacity in a consensus instance
	IoBufSize     int    // the size of each underlying buffer in bufio.Reader and bufio.Writer
	TcpBufSize    int    // the size of each TCP write buffer and TCP read buffer
	KeyLen        int    // the length of KV-store key string
	ValLen        int    // the length of KV-store value string

	SvrLogInterval      time.Duration // a server logger's sleep time after generating a log
	ClientLogInterval   time.Duration // a client logger's sleep time after generating a log
	ClientTimeout       time.Duration // closed-loop only, a client exits after ClientTimeout
	ConsensusStartAfter time.Duration // after this time, the consensus executor will start working (this variable is for saturating the system with open-loop clients)
	StorageMode         int           // 0: the dictionary KV store, 1: Redis GET&SET, 2: Redis MGET&MSET
	RedisAddr           []string      // only used when StorageMode is 1 or 2
}

func (c *Config) LoadConfigs() {
	c.loadEnvVars1()
	c.loadEnvVars2()
	c.CalcConstants()
	c.loadRedisVars()
}

func (c *Config) loadEnvVars1() {
	c.Role = os.Getenv("RC_Role")
	c.Id = os.Getenv("RC_Index")

	c.SvrIp = os.Getenv("RC_SvrIp")
	c.ProxyPort = os.Getenv("RC_PPort")
	c.NetworkPort = os.Getenv("RC_NPort")
	c.Peers = strings.Split(os.Getenv("RC_Peers"), " ")

	c.ProxyAddr = os.Getenv("RC_Proxy")
}

func (c *Config) loadEnvVars2() {
	c.ControllerAddr = getEnvStr("RC_Ctrl")
	c.ProjectFolder = getEnvStr("RC_Folder")
	c.LogLevel = getEnvStr("RC_LLevel")
	c.ClosedLoop = strToBool(os.Getenv("Rabia_ClosedLoop"), true) // for both servers and clients

	Conf.NServers = getEnvInt("Rabia_NServers")
	Conf.NFaulty = getEnvInt("Rabia_NFaulty")
	Conf.NClients = getEnvInt("Rabia_NClients")
	Conf.NConcurrency = 1

	Conf.ProxyBatchSize = getEnvInt("Rabia_ProxyBatchSize")
	Conf.ProxyBatchTimeout = time.Duration(getEnvInt("Rabia_ProxyBatchTimeout")) * time.Millisecond
	Conf.NetworkBatchSize = getEnvInt("Rabia_NetworkBatchSize")
	Conf.NetworkBatchTimeout = time.Duration(getEnvInt("Rabia_NetworkBatchTimeout")) * time.Millisecond

	Conf.ClientBatchSize = getEnvInt("Rabia_ClientBatchSize")
	Conf.ClientTimeout = time.Duration(getEnvInt("Rabia_ClientTimeout")) * time.Second
	Conf.ClientThinkTime = getEnvInt("Rabia_ClientThinkTime")
	Conf.NClientRequests = getEnvInt("Rabia_ClientNRequests")
}

func (c *Config) CalcConstants() {
	c.NMinusF = c.NServers - c.NFaulty
	c.Majority = c.NServers/2 + 1
	c.MajorityPlusF = c.NServers/2 + c.NFaulty + 1
	c.FaultyPlusOne = c.NFaulty + 1

	if c.NClientRequests == 0 {
		c.NClientRequests = 10000000 // the default value
	}
	c.LenLedger = 10000
	c.LenBlockArray = 10
	c.LenChannel = 500000

	c.IoBufSize = 4096 * 4000
	c.TcpBufSize = 7000000
	c.KeyLen = 8
	c.ValLen = 8

	c.SvrLogInterval = 4 * time.Second
	c.ClientLogInterval = 15 * time.Second
	c.ConsensusStartAfter = 0 * time.Second // for open-loop testings
}

func (c *Config) loadRedisVars() {
	c.StorageMode = 0
	c.RedisAddr = []string{"localhost:6379", "localhost:6380", "localhost:6381"}
}

func getEnvStr(key string) string {
	return os.Getenv(key)
}

func getEnvInt(key string) int {
	ret, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		panic(fmt.Sprint("env var loading error", key, err))
	}
	return ret
}

/*
	Convert a string to an integer array
	str: the input string, integers in the string are separated by spaces (e.g., "1 3 5 7 9")
*/
func strToIntArray(str string) []int {
	strs := strings.Split(str, " ")
	ints := make([]int, len(strs))
	var err error
	for i := range ints {
		ints[i], err = strconv.Atoi(strs[i])
		if err != nil {
			panic(err)
		}
	}
	return ints
}

// Convert a string to a bool
func strToBool(str string, defaultVal bool) bool {
	res, err := strconv.ParseBool(str)
	if err != nil {
		return defaultVal
	}
	return res
}
