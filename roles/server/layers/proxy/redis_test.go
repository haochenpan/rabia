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
package proxy

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"net"
	"os"
	"rabia/internal/config"
	"rabia/internal/message"
	"testing"
	"time"
)

var ctx = context.Background()

/*
	requireRedis skips the test when a Redis server is not reachable at one of addrs. When the environment variable
	RABIA_REQUIRE_REDIS is set (as it is in CI), an unreachable server fails the test instead.
*/
func requireRedis(t *testing.T, addrs ...string) {
	t.Helper()
	for _, addr := range addrs {
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err != nil {
			if os.Getenv("RABIA_REQUIRE_REDIS") != "" {
				t.Fatalf("redis is not reachable at %s: %v", addr, err)
			}
			t.Skipf("redis is not reachable at %s (set RABIA_REQUIRE_REDIS=1 to fail instead): %v", addr, err)
		}
		_ = conn.Close()
	}
}

func TestRedis(t *testing.T) {
	requireRedis(t, "localhost:6379")
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	err := rdb.Set(ctx, "key", "value", 0).Err()
	if err != nil {
		panic(err)
	}

	val, err := rdb.Get(ctx, "key").Result()
	if err != nil {
		panic(err)
	}
	if val != "value" {
		t.Error("error here 1")
	}
	t.Log("key", val)

	val2, err := rdb.Get(ctx, "key2").Result()
	if err == redis.Nil {
		fmt.Println("key2 does not exist")
	} else if err != nil {
		panic(err)
	} else {
		fmt.Println("key2", val2)
	}
	// Output: key value
	// key2 does not exist
}

func TestProxy_RedisBatchExecuteCmd(t *testing.T) {
	requireRedis(t, "localhost:6379", "localhost:6380", "localhost:6381")
	config.Conf.StorageMode = 2
	config.Conf.RedisAddr = []string{"localhost:6379", "localhost:6380", "localhost:6381"}
	config.Conf.ProxyBatchSize = 5
	config.Conf.ClientBatchSize = 2
	config.Conf.KeyLen = 2
	config.Conf.ValLen = 2

	p := Proxy{
		RedisClient: RedisInit(0),
		RedisCtx:    context.Background(),
	}

	dec1 := &message.ConsensusObj{
		CliIds:  []uint32{0, 0, 0, 1, 1},
		CliSeqs: []uint32{1000, 1001, 1002, 2000, 2001},
		Commands: []string{"0k1v1", "1k2", "0k3v3", "1k4", "0k5v5",
			"1k6", "0k7v7", "1k3", "0k9v9", "1k1"},
	}

	p.CurrDec = dec1
	res := p.RedisBatchExecuteCmd()
	for _, re := range res {
		fmt.Println(re)
	}

	dec2 := &message.ConsensusObj{
		CliIds:  []uint32{1, 1, 1, 2, 2},
		CliSeqs: []uint32{2002, 2003, 2004, 1000, 1001},
		Commands: []string{"0k2v2", "1k2", "0k4v4", "1k4", "0k5v5",
			"1k5", "0k6v6", "1k6", "0k7v7", "1k7"},
	}
	p.CurrDec = dec2
	res = p.RedisBatchExecuteCmd()
	for _, re := range res {
		fmt.Println(re)
	}

}