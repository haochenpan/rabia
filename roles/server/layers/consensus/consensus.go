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
	The consensus package defines the struct of a consensus instance and methods shared by the consensus executor and
	the message handler. Both routines modify states in the server Ledger, but the executor follows the pseudo-code of
	Rabia. The handler does the dirty work that is not described in the pseudo-code -- basically, send and receive
	messages on the behalf of this executor (but the proposal request and reply phase is an exception). See descriptions
	in executor.go and msg_handler.go for more details.

	Note:

	1. The implemented code for deciding each slot is somewhat different from the algorithm in our paper; The
	implementation follows a more verbose version of the algorithm presented in the SOSP paper, see the document in the
	docs folder
*/
package consensus

import (
	"container/heap"
	"fmt"
	"github.com/rs/zerolog"
	"math/rand"
	"os"
	. "rabia/internal/config"
	"rabia/internal/ledger"
	"rabia/internal/logger"
	. "rabia/internal/message"
	"rabia/internal/queue"
	"sync"
)

/*
	Race Conditions Documented

	The correct execution of Rabia assumes a few timing issues documented below never occur. In fact, given the
	right/default user parameters, these issues are extremely unlikely to happen and they never happened in any
	testing or benchmarking runs.

	Why we claim they virtually cannot occur:

	These timing issues are there because we use a circular ring buffer, and when one of the MsgHandler/Executor resets
	a slot while the other access to the slot, problematic results may get returned. Since our default configuration
	assumes a long ledger (10000+ slots), slots are reset are far away from slots are currently accessed, those timing
	issues never had a chance to occur.

	Two cases where race condition may occur (extremely unlikely by the default parameter):

	1. In MsgHandler's function binConMsgHandling(), the routine may call UpdateTermIfNecessary(), which may reset the
	slot X. However, the Executor may call putBackMyProposal() in the epilogue() function, which may access fields of
	slot X.

	2. In Executor's function getRequest(), the routine may call UpdateTermIfNecessary(), which may reset the slot X.
	However, the MsgHandler may access slot X after when it receives a ProposalRequest.

	How to eradicate this issue:

	When modifying or reading Term, IsDone, and Decision fields, acquire Slot.lock first to avoid concurrent writing,
	undesirable resetting or data overriding. The downside of this modification is this may slow down Rabia. In future
	versions of Rabia, we will tackle this issue.
*/

/*
	For recording the instance's runtime statistics
	maybe: use Stat in the future
*/
type Stat struct {
	/*
		NumOfRounds:
		the i-th entry stores the number of consensus rounds used in deciding slot i, this number may vary from server
		to server: those server who learn the slot decision from other servers may have a higher value in the entry.

		DecisionTyp:
			0 -- Matched Slots: the decision is not null and my proposal = the decision
			1 -- Unmatched Slots: the decision is not null and my proposal != the decision
			2 -- Null Slots: the decision is null
	*/
	NumOfRounds []uint32
	DecisionTyp []uint32 // 0: a matched slot, 1: a unmatched slot, 2: a null slot
	NumOfCliReq []int    // the number of client-batched requests in this slot
	/*
		These two numbers are calculated just before the program exits.
		TotalSlots: the sum of Matched Slots, Unmatched Slots, and Null Slots
		TotalRounds: the sum of numbers in NumOfRounds entries
	*/

	TotalRounds  uint32
	TotalSlots   uint32
	NotNullSoFar int // the number of matched slots + the number of unmatched slots decided so far
	CliReqSoFar  int // the number of client-batched requests that have been made into decided slots so far
	/*
		Tally the msg with "bad" terms. May increase as the consensus instance executes
		Definition: the term of a message -- for a message that is associated with a logical slot number, the term of
		the message is the term of the Slot object that represents the logical slot.
		Example: when LenCommandsArray (the Ledger's length) = 10:
			for logical slot 5, the term is 0;
			for logical slot 9, the term is 0;
			for logical slot 15, the term is 1;
			for logical slot 19, the term is 1;
		Say a message's term is x, and the the current term of the Slot object is y:
		A msg's term is bad if x < y or x > y + 1 (strictly one more).
		Otherwise, when x = y or x = y + 1, the term is not bad.
	*/
	BadTermMsg int
}

type Consensus struct {
	SvrId uint32          // the server id
	InsId uint32          // the consensus instance id
	Wg    *sync.WaitGroup // for the proxy to mark it has exited
	Done  chan struct{}   // for the server to signal the instance (and other layers and instances) to exit

	NetToMsgHandler  chan Msg // receives ClientRequest, Proposal, State, Vote, ProposalRequest, and Decision
	MsgHandlerToNet  chan Msg // sends ProposalReply
	NetToConExecutor chan Msg // receives ProposalReply
	ConExecutorToNet chan Msg // sends ProposalRequest, Proposal, State, Vote, and Decision

	Queue queue.PQueue
	QLock *sync.Mutex

	SvrSeq int // the slot # currently working on
	Ledger ledger.Ledger
	Coin   *rand.Rand // the common coin used in the algorithm

	/*
		Discard: if it turns out that my proposal != the decision, the consensus object's proxy id and proxy sequence of
		the decision is recorded in the Discard dictionary. The next time we see the consensus object in the pending
		queue, we discard the decision.
	*/
	Discard map[string]bool
	Logger  zerolog.Logger // consensus layer level logger
	LogFile *os.File       // the log file that should be called .Sync() method before the routine exits

	NormalSlots, UnmatchedSlots, NullSlots    int    // num. of non-discarded, discarded, and null slots so far,
	TotalRounds                               uint32 // num. of rounds so far,
	TotalSlots                                int    // the sum of NormalSlots, UnmatchedSlots, NullSlots, calculated before exit
	OlderThanTermMsg                          int    // num. of msgs that have terms older than the current slot's term
	CurrConsecutiveNulls, MaxConsecutiveNulls int    //
	MaxConsecutiveNullsEndSeq                 int    //
	NumClientBatchedRequests                  int    // the number of client-batched requests that have been decided

	NumOfRoundsDist []int // index: num of rounds, element: frequency

	Per1000RoundDist []int          // for each 1000 slots, log the number of rounds distribution to roundDist log files
	RoundDistLogger  zerolog.Logger // roundDist logger
	RoundDistLogFile *os.File       //
}

/*
	Initialize a consensus instance
*/
func ConsensusInit(svrId, insId uint32, done chan struct{}, doneWg *sync.WaitGroup, netToMsgHandler, msgHandlerToNet,
	netToConExecutor, conExecutorToNet chan Msg, ledger ledger.Ledger) *Consensus {
	zerologger0, logFile0 := logger.InitLogger("consensus", svrId, insId, "file")
	zerologger2, logFile2 := logger.InitLogger("roundDist", svrId, insId, "file")

	c := &Consensus{
		SvrId: svrId,
		InsId: insId,
		Wg:    doneWg,
		Done:  done,

		NetToMsgHandler:  netToMsgHandler,
		MsgHandlerToNet:  msgHandlerToNet,
		NetToConExecutor: netToConExecutor,
		ConExecutorToNet: conExecutorToNet,

		Queue: make(queue.PQueue, 0),
		QLock: &sync.Mutex{},

		SvrSeq: -1,
		Ledger: ledger,

		Discard: make(map[string]bool),
		Logger:  zerologger0,
		LogFile: logFile0,

		NumOfRoundsDist: make([]int, Conf.LenBlockArray*2+2),
		/*
			the length of the array above is capped
			why + 2: if old = 20, new = 21, then [21] is used (see comments in executor.go)
		*/

		Per1000RoundDist: make([]int, Conf.LenBlockArray*2+1),
		RoundDistLogger:  zerologger2,
		RoundDistLogFile: logFile2,
	}
	heap.Init(&c.Queue)
	return c
}

/*
	Determines whether the consensus instance needs to update (the term of) a Slot object when receiving a new message
	object. See comments near the code for detailed explanations.

	seq: the logical slot's sequence embedded in a incoming message
	pan: if the incoming message is associated with a bad term (see below), triggers a panic

	It returns a boolean that indicates whether the term of the message is of a not-bad term, it is of a not-bad term if
	the message's term = the slot's current term or the message's term = (the slot's current term + 1).
*/
func (c *Consensus) UpdateTermIfNecessary(seq uint32, pan bool) (ret bool) {
	slot := seq % Conf.LenLedger
	term := seq / Conf.LenLedger
	if term == c.Ledger[slot].Term {
		return true // same term, proceed
	} else if term == c.Ledger[slot].Term+1 {
		// 1 term higher than the current term, update term and then proceed
		c.Ledger[slot].Lock.Lock()
		if term == c.Ledger[slot].Term+1 {
			c.Ledger[slot].Reset()
			c.Ledger[slot].Term = term
		}
		c.Ledger[slot].Lock.Unlock()
		return true // term updated, proceed
	} else {
		// message is older than or > 1 newer than the current term, don't proceed
		if pan {
			panic("should not happen, function UpdateTermIfNecessary was asked to trigger a panic statement")
		}
		return false
	}
}

/*
	Panics if the term associated with seq is not equal to the slot's current term.
*/
func (c *Consensus) PanicTermNotMatched(seq uint32) {
	if !c.IsTermMatched(seq) {
		slot := seq % Conf.LenLedger
		term := seq / Conf.LenLedger
		panic(fmt.Sprintf("should not happen: seq=%d, term=%d, c.Ledger[slot].Term=%d",
			seq, term, c.Ledger[slot].Term))
	}
}

/*
	Returns false if the term associated with seq is not equal to the slot's current term.
	Otherwise, return true
*/
func (c *Consensus) IsTermMatched(seq uint32) bool {
	slot := seq % Conf.LenLedger
	term := seq / Conf.LenLedger
	if term == c.Ledger[slot].Term {
		return true
	}
	return false
}

/*
	Pushes a ConsensusObj to the pending request queue
*/
func (c *Consensus) QPush(obj ConsensusObj) {
	c.QLock.Lock()
	heap.Push(&c.Queue, obj)
	c.QLock.Unlock()
}

/*
	Pops a ConsensusObj from the pending request queue if there are any requests
*/
func (c *Consensus) QPop() (ConsensusObj, bool) {
	c.QLock.Lock()
	defer c.QLock.Unlock()
	if c.Queue.Len() == 0 {
		return ConsensusObj{}, false
	} else {
		return heap.Pop(&c.Queue).(ConsensusObj), true
	}
}

/*
	Removes trailing zeros in an integer array, e.g., [1, 0, 1, 0] can produce [1, 0, 1] after the function call
*/
func RemoveTrailingZeros(array []int) []int {
	endIndex := len(array)
	for i := len(array) - 1; i >= 0; i-- {
		if array[i] != 0 {
			break
		}
		endIndex = i
	}
	return array[:endIndex]
}

/*
	The total frequency should equal to the sum of NormalSlots, UnmatchedSlots, NullSlots when the executor exits
*/
func (c *Consensus) freqSum() int {
	acc := 0
	for _, v := range c.NumOfRoundsDist {
		acc += v
	}
	return acc
}

/*
	Find the number of rounds given a percentile (e.g. 50, 95, 99)
*/
func (c *Consensus) findRds(percentile int) int {
	acc := 0
	sum := c.freqSum()
	for i, v := range c.NumOfRoundsDist {
		acc += v
		if float64(acc) >= float64(percentile)*float64(sum)/100 {
			return i
		}
	}
	panic("should not happen, program's logic error")
}

func (c *Consensus) CommonCoinFlip() uint32 {
	return uint32(c.Coin.Intn(2))
}

func (c *Consensus) ResetCommonCoin() {
	c.Coin = rand.New(rand.NewSource(42))
}
