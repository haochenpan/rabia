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
package ledger

import (
	"rabia/internal/config"
	"rabia/internal/message"
	"sort"
	"sync"
)

/*
	1. Package Description

	The ledger package defines the Ledger object, and exactly one copy of a Ledger is held by each server. A Ledger is
	made by an array of Slot objects, and the Slot associated with index i (of the ledger array) stores all current
	information this server holds regarding the replicated log entry i, including: 1) this server's proposal for this
	replicated log entry and this server's binary consensus messages, 2) other servers' proposals and binary consensus
	messages, 3) whether a decision message has received, 4) whether this server has made a decision, 5) what is the
	decision, 6) what's the current phase and round of the consensus protocol that this server has proceeded so far. Due
	to the network latency and other factors, each server may have (slightly) different views about a replicated log
	entry at a certain time, so it is natural that they hold (slightly) different values in some fields of a Slot
	object. Nevertheless, Rabia's goal is to let all servers in the cluster agree on the same sequence of decisions
	eventually, and as fast as possible

	Note: At each server, the ledger is read by the proxy layer and read and written by the consensus layer.

	2. About ConsensusObj

	A ConsensusObj is uniquely identified by its ProId and ProSeq fields; if two objects' ProId and ProSeq fields are
	identical, they are considered as equal. This has been discussed in the msg package.

	3. The Majority Value and the Majority Tally

	RecvProposals' majority value (MajV): the proposal (consensus object) that occurred most often. If two proposals
	have the same number of occurrences, then the consensus object that wins the less-than relation is the majority
	value.

	RecvProposals' majority value's tally (MajT): the occurrences of MajV

	RecvBCMsgs' majority value (MajV): Is either 0 or 1, depends on which occurs more. If then occur evenly, then this
	function prefers to output 1

	RecvBCMsgs' majority value's tally (MajT): the occurrences of MajV
*/

// A Tally object counts the number of occurrences of a proposal
type Tally struct {
	Proposal message.ConsensusObj // uniquely identified by its ProId and ProSeq fields.
	Count    int                  // increment-only for the purpose of tallying
}

type Slot struct {
	Term       uint32               // the num of times that this slot has been reused/reset
	Lock       sync.Mutex           // prevents undesirable concurrent slot operations, see comments below (important!)
	IsDone     bool                 // whether a decision has been generated
	HasRecvDec bool                 // whether a decision msg is received, and it guarantees <=1 Decision msg goes into the Queue
	Decision   message.ConsensusObj // if IsDone == true, this field saves the decision for the current term
	Queue      chan message.Msg     // from Msg Handler to Executor
	Phase      uint32               // current phase
	Round      uint32               // current round

	/*
		Lock: prevents undesirable concurrent reset. Also see the Race Conditions Documented section in consensus.go

		MyProposal: does not change after a variable assignment
		RecvProposals: the order of elements is not stable -- some functions may sort the Tally objects
		MyBCMsgs: stands for "my binary consensus messages".
			MyBCMsgs[p][0] stores the STATE of phase p, round 1
			MyBCMsgs[p][1] stores the VOTE of phase p, round 2
		RecvBCMsgs: stands for "received binary consensus messages".
			RecvBCMsgs[p][r][0] counts the number of 0s received at phase p, round r+1
			RecvBCMsgs[p][r][1] counts the number of 1s received at phase p, round r+1
			RecvBCMsgs[p][0][2] is not used
			RecvBCMsgs[p][1][2] counts the number of ?s received at phase p, round 2
		RecvBCMsgsT: stands for "received messages' tallies"
			RecvBCMsgsT[p][0] counts the number of 0s and 1s received at phase p, round 1
			RecvBCMsgsT[p][1] counts the number of 0s, 1s, and ?s received at phase p, round 2

		Note: when updating an RecvBCMsgs entry, also update the corresponding RecvBCMsgsT entry
	*/
	MyProposal    message.ConsensusObj // this server's proposal at phase 0
	RecvProposals []Tally              // received proposals at phase 0 and their occurrences
	MyBCMsgs      [][2]uint32          // this server's state and vote messages
	RecvBCMsgs    [][2][3]int          // received state and vote messages
	RecvBCMsgsT   [][2]int             // received state and vote messages' tallies
}

type Ledger []*Slot

/*
 this function resets or initializes every fields but leaves the Term and Lock fields unchanged. Please acquire the Lock
 before resetting the slot, and increment the Term variable while holding the lock.

 Note: Term and Lock are initialized at object allocation phase -- Lock is an allocated lock object but not a pointer.
*/
func (s *Slot) Reset() {
	s.IsDone = false
	s.HasRecvDec = false
	s.Decision = message.ConsensusObj{}
	s.Queue = make(chan message.Msg, 10)
	s.Phase = 0
	s.Round = 0

	s.MyProposal = message.ConsensusObj{}
	s.RecvProposals = make([]Tally, 0)
	s.MyBCMsgs = make([][2]uint32, config.Conf.LenBlockArray)
	s.RecvBCMsgs = make([][2][3]int, config.Conf.LenBlockArray)
	s.RecvBCMsgsT = make([][2]int, config.Conf.LenBlockArray)
}

// Increases the Phase variable by one and decreases the Round variable by one
func (s *Slot) IncrPhaseDecrRound() {
	s.Phase++
	s.Round--
}

// MyProposal setter
func (s *Slot) SetMyProposal(p message.ConsensusObj) {
	s.MyProposal = p
}

// MyProposal getter
func (s *Slot) GetMyProposal() message.ConsensusObj {
	return s.MyProposal
}

// RecvProposals setter
func (s *Slot) PutRecvProposals(p message.ConsensusObj) {
	for i := 0; i < len(s.RecvProposals); i++ {
		if message.ProxySeqIdEqual(&s.RecvProposals[i].Proposal, &p) {
			s.RecvProposals[i].Count++
			s.RecvBCMsgsT[0][0]++
			return
		}
	}
	s.RecvProposals = append(s.RecvProposals, Tally{p, 1})
	s.RecvBCMsgsT[0][0]++
}

/*
	Sort RecvProposals according to their occurrences so that the fist proposal after sorting is the majority value,
	if two proposals have the same number of occurrences, put the one wins the less-than relation to the front.
*/
func (s *Slot) SortRecvProposals() {
	sort.Slice(s.RecvProposals, func(i, j int) bool {
		return s.RecvProposals[i].Count > s.RecvProposals[j].Count ||
			(s.RecvProposals[i].Count == s.RecvProposals[j].Count &&
				message.ProxySeqIdLessThan(&s.RecvProposals[i].Proposal, &s.RecvProposals[j].Proposal))
	})
}

// RecvProposals' majority value getter
func (s *Slot) RecvProposalsMajV() message.ConsensusObj {
	s.SortRecvProposals()
	return s.RecvProposals[0].Proposal
}

//RecvProposals' majority tally getter
func (s *Slot) RecvProposalsMajT() int {
	s.SortRecvProposals()
	return s.RecvProposals[0].Count
}

// MyBCMsgs setter
func (s *Slot) SetMyBCMsgs(phase, round, x uint32) {
	s.MyBCMsgs[phase][round-1] = x
}

// MyBCMsgs getter (MyBCMsgs[p][0] stores my round 1 msg, MyBCMsgs[p][1] stores my round 2 msg)
func (s *Slot) GetMyBCMsgs(phase, round uint32) uint32 {
	return s.MyBCMsgs[phase][round-1]
}

// RecvBCMsgs setter (note: RecvBCMsgsT also needs to be updated)
func (s *Slot) PutRecvBCMsgs(phase, round, x uint32) {
	s.RecvBCMsgs[phase][round-1][x]++
	s.RecvBCMsgsT[phase][round-1]++
}

/*
	RecvBCMsgs' majority value getter (when RecvBCMsgs[p][0] == RecvBCMsgs[p][1], this function prefers to
	output 1, as implied in the algorithm)
*/
func (s *Slot) RecvBCMsgsMajV(phase, round uint32) uint32 {
	if s.RecvBCMsgs[phase][round-1][0] > s.RecvBCMsgs[phase][round-1][1] {
		return 0
	} else {
		return 1
	}
}

/*
	RecvBCMsgs' majority tally getter (when RecvBCMsgs[p][0] == RecvBCMsgs[p][1], this function prefers to
	output RecvBCMsgs[p][1], as implied in the algorithm)
*/
func (s *Slot) RecvBCMsgsMajT(phase, round uint32) int {
	if s.RecvBCMsgs[phase][round-1][0] > s.RecvBCMsgs[phase][round-1][1] {
		return s.RecvBCMsgs[phase][round-1][0]
	} else {
		return s.RecvBCMsgs[phase][round-1][1]
	}
}

// RecvBCMsgsT getter
func (s *Slot) GetRecvBCMsgsT(phase, round uint32) int {
	return s.RecvBCMsgsT[phase][round-1]
}

/*
	Determines whether enough messages have received in order to proceed the executor (note: this function assumes
	Conf.NMinusF is initialized). Enough messages means no less than n-f messages.
*/
func (s *Slot) HasEnoughMsg(phase, round uint32) bool {
	return s.RecvBCMsgsT[phase][round-1] >= config.Conf.NMinusF
}
