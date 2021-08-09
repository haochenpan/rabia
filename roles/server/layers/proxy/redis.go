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
	"github.com/go-redis/redis/v8"
	. "rabia/internal/config"
)

func RedisInit(id uint32) *redis.Client {
	if Conf.StorageMode == 1 || Conf.StorageMode == 2 {
		return redis.NewClient(&redis.Options{
			Addr:     Conf.RedisAddr[id],
			Password: "", // no password set
			DB:       0,  // use default DB
		})
	}
	return nil
}

/*
	Execute the KV store command and assemble a reply to return
*/

func (p *Proxy) redisExecuteCmd(cmd string) string {
	typ := cmd[0:1]
	key := cmd[1 : 1+Conf.KeyLen]
	val := cmd[1+Conf.KeyLen:]
	if typ == "0" { // write
		if err := p.RedisClient.Set(p.RedisCtx, key, val, 0).Err(); err != nil {
			panic(err)
		}
		return "0" + key + "ok"
	} else { // read
		v, err := p.RedisClient.Get(p.RedisCtx, "key2").Result()
		if err == redis.Nil {
			return "1" + key
		} else if err != nil {
			panic(err)
		} else {
			return "1" + key + v
		}
	}
}

func (p *Proxy) RedisBatchExecuteCmd() [][]string {
	dec := p.CurrDec
	mset := make([]string, Conf.ClientBatchSize*Conf.ProxyBatchSize*2) // pending MSET requests
	mget := make([]string, Conf.ClientBatchSize*Conf.ProxyBatchSize)   // pending MGET requests
	msetCtr, mgetCtr := 0, 0                                           // elements counter

	replies := make([][]string, len(dec.CliIds)) // an array of client replies
	repliesI, repliesJ := 0, 0                   // indexing the 2D array

	for idx := range dec.CliIds {
		replies[idx] = make([]string, Conf.ClientBatchSize) // initialize the 1D array
		// for each client's requests
		for j, cmd := range dec.Commands[idx*Conf.ClientBatchSize : idx*Conf.ClientBatchSize+Conf.ClientBatchSize] {
			typ := cmd[0:1]
			key := cmd[1 : 1+Conf.KeyLen]
			val := cmd[1+Conf.KeyLen:]
			if typ == "0" { // write
				mset[msetCtr] = key
				msetCtr++
				mset[msetCtr] = val
				msetCtr++
				replies[idx][j] = "0" + key + "ok" // if write op, set a reply (MSET always success)
			} else { // read
				mget[mgetCtr] = key
				mgetCtr++
				// if a read op, do not set a reply
			}
		}
	}

	//fmt.Println(msetCtr, mset[:msetCtr])
	//fmt.Println(mgetCtr, mget[:mgetCtr])

	// execute MSET
	if msetCtr != 0 {
		if err := p.RedisClient.MSet(p.RedisCtx, mset[:msetCtr]).Err(); err != nil {
			panic(err)
		}
	}

	// execute MGET
	if mgetCtr != 0 {
		vs, err := p.RedisClient.MGet(p.RedisCtx, mget[:mgetCtr]...).Result()
		if err != nil {
			panic(err)
		} else {

			for _, v := range vs {
				for ; len(replies[repliesI][repliesJ]) != 0; { // if not a read op
					/*
						find the next read op
					*/
					repliesJ++
					if repliesJ == Conf.ClientBatchSize {
						repliesJ = 0
						repliesI++
					}
				}

				//if replies[repliesI][repliesJ] should be a read op
				//req is the original request's OP field + key field
				req := dec.Commands[repliesI*Conf.ClientBatchSize+repliesJ][:1+Conf.KeyLen]
				if v == nil { // if the key does not exists
					replies[repliesI][repliesJ] = req
				} else { // if the key exists
					if rep, ok := v.(string); !ok {
						panic(v)
					} else {
						replies[repliesI][repliesJ] = req + rep
					}
				}
			}
		}
	}

	return replies
}