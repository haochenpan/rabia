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
	The queue package defines an implementation of the ConsensusObj priority queue, which is used to sort (based on the
	proxy id and proxy sequence fields) and store pending proxy-batched requests.

	Note:
	1. A proxy-batched request contains one or more client-batched requests that are batched again by the proxy. The
	size (number of requests) of the batch depends on several parameters in Conf and the arrival time of client
	requests.
	2. Each consensus instance has its pending request queue, which means a server can have more than one queue when
	Concurrency is greater than 1.
	3. The priority queue follows the example at https://golang.org/pkg/container/heap
*/
package queue

import "rabia/internal/message"

type PQueue []message.ConsensusObj

func (q PQueue) Len() int {
	return len(q)
}

//  The priority is defined by the ProxySeqIdLessThan function (see that function's comment)
func (q PQueue) Less(i, j int) bool {
	return message.ProxySeqIdLessThan(&q[i], &q[j])
}

func (q PQueue) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
}

/*
	Note: suppose q is a PQueue object, do not call q.Push and q.Pop directly, instead, call heap.Push and heap.Pop,
	which calls q.Push or q.Pop and performs sorting.

	Push and Pop use pointer receivers because they modify the slice's length, not just its contents.
*/
func (q *PQueue) Push(x interface{}) {
	*q = append(*q, x.(message.ConsensusObj))
}

func (q *PQueue) Pop() interface{} {
	old := *q
	n := len(old)
	x := old[n-1]
	//old[n-1] = nil // avoid memory leak (only used if PQueue is defined as []*ConsensusObj)
	*q = old[0 : n-1]
	return x
}
