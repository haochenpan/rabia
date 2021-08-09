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
package consensus

import (
	"fmt"
	. "rabia/internal/config"
	. "rabia/internal/message"
	"time"
)

/*
	The consensus executor's main function, mainly an event-driven (incoming messages) for-loop. See more comments in
	called functions. After the for-loop, it counts statistics and write them to the log file.

	Special notes on ProposalRequest and ProposalReply (again)
	ProposalRequest:
 	Phase: SvrId (the source server's id), Value: the sequence number of the proposal
	ProposalReply:
 	Phase: the destination server's id, Value: the sequence number of the proposal
*/
func (c *Consensus) Executor() {
	time.Sleep(Conf.ConsensusStartAfter)

	defer c.Wg.Done()
MainLoop:
	for {
		select {
		case <-c.Done:
			break MainLoop
		default:
		}

		if proceed := c.getRequest(); !proceed {
			continue
		}

		seq := uint32(c.SvrSeq)
		c.phase0Round1BeforeWait(seq)
		if !c.wait(seq) {
			continue
		}
		dec, ret := c.phase0Round1AfterWait(seq)
		if ret {
			c.epilogue(seq, dec)
			continue
		}

		c.phase0Round2BeforeWait(seq)
		if !c.wait(seq) {
			continue
		}
		dec, ret = c.phase0Round2AfterWait(seq)
		if ret {
			c.epilogue(seq, dec)
			continue
		}

		for {
			c.phaseNRound1BeforeWait(seq)
			if !c.wait(seq) {
				continue MainLoop
			}
			dec, ret := c.phaseNRound1AfterWait(seq)
			if ret {
				c.epilogue(seq, dec)
				continue MainLoop
			}

			c.phaseNRound2BeforeWait(seq)
			if !c.wait(seq) {
				continue MainLoop
			}
			dec, ret = c.phaseNRound2AfterWait(seq)
			if ret {
				c.epilogue(seq, dec)
				continue MainLoop
			}
		}
	}

	c.logExitStatus()
	if err := c.LogFile.Sync(); err != nil {
		panic(fmt.Sprint("should not happen", err))
	}
	if err := c.RoundDistLogFile.Sync(); err != nil {
		panic(fmt.Sprint("should not happen", err))
	}
}

/*
	Generates a Proposal message by fetching the MyProposal field in the Ledger slot
*/
func (c *Consensus) genProposalMsg(seq uint32) Msg {
	c.PanicTermNotMatched(seq)
	slot := seq % Conf.LenLedger
	obj := c.Ledger[slot].GetMyProposal()
	obj.SvrSeq = seq
	msg := Msg{Type: Proposal, Obj: &obj}
	return msg
}

/*
	Generates a State or Vote message by fetching the MyBiConMsg field in the Ledger slot
*/
func (c *Consensus) genBinConMsg(seq, pse, rod uint32) Msg {
	c.PanicTermNotMatched(seq)
	slot := seq % Conf.LenLedger
	if rod == 1 {
		msg := Msg{Phase: pse, Type: State, Value: c.Ledger[slot].GetMyBCMsgs(pse, rod)}
		msg.Obj = &ConsensusObj{SvrSeq: seq}
		return msg
	} else if rod == 2 {
		msg := Msg{Phase: pse, Type: Vote, Value: c.Ledger[slot].GetMyBCMsgs(pse, rod)}
		msg.Obj = &ConsensusObj{SvrSeq: seq}
		return msg
	} else {
		panic(fmt.Sprint("should not happen, error rod number"))
	}
}

/*
	Generates a Decision message by concluding information (i.e., the current majority value) from the Ledger
*/
func (c *Consensus) genDecMsgType1(seq uint32) Msg {
	c.PanicTermNotMatched(seq)
	slot := seq % Conf.LenLedger
	obj := c.Ledger[slot].RecvProposalsMajV()
	obj.SvrSeq = seq
	msg := Msg{Type: Decision, Obj: &obj}
	return msg
}

/*
	Generates a Decision message by concluding information (i.e., the consensus object) from an incoming Decision
	message
*/
func (c *Consensus) genDecMsgType2(seq uint32, obj ConsensusObj) Msg {
	obj.SvrSeq = seq
	msg := Msg{Type: Decision, Obj: &obj}
	return msg
}

/*
	Generates a ProposalReply message to answer a particular ProposalRequest.
	Note: call this function only when it is safe to reply:
		1. the slot holds >= n - f proposal, and
		2. the majority value's count >= n / 2 + 1
*/
func (c *Consensus) genProposalReply(seq uint32, dst uint32) Msg {
	c.PanicTermNotMatched(seq)
	slot := seq % Conf.LenLedger
	obj := c.Ledger[slot].RecvProposalsMajV()
	obj.SvrSeq = seq
	msg := Msg{Phase: dst, Type: ProposalReply, Obj: &obj, Value: seq}
	return msg
}

/*
	Waits enough Proposal, State, or Vote message or jumps out of the process of deciding the current slot if a Decision
	message is received
*/
func (c *Consensus) wait(seq uint32) bool {
	c.PanicTermNotMatched(seq)
	slot := seq % Conf.LenLedger
	for {
		select {
		case <-c.Done:
			return false
		case msg := <-c.Ledger[slot].Queue:
			switch msg.Type {
			case Proposal, State, Vote:
				if c.Ledger[slot].HasRecvDec { // if has received a decision message, discard this message
					continue
				}
				if msg.Phase != c.Ledger[slot].Phase {
					panic("should not happen 1 (reason see the if case)")
				}
				if c.Ledger[slot].Round == 1 && c.Ledger[slot].Phase == 0 && msg.Type != Proposal {
					panic("should not happen 2 (reason see the if case)")
				} else if c.Ledger[slot].Round == 1 && c.Ledger[slot].Phase != 0 && msg.Type != State {
					panic("should not happen 3 (reason see the if case)")
				} else if c.Ledger[slot].Round == 2 && msg.Type != Vote {
					panic("should not happen 4 (reason see the if case)")
				}
				return true // should continue the program execution

			case Decision:
				if c.Ledger[slot].IsDone {
					panic("should not happen 5 (reason see the if case)")
				}
				/*
					Even if the server receives a decision message before reaching a round-based consensus with other
					servers, the number of rounds should be calculated from the variables (Phase and Round) of this
					server.
				*/
				c.epilogue(seq, *msg.Obj)
				return false // should start to decide the next slot
			}
		}
	}
}

/*
	The code for Phase 0, Round 1, before waiting for enough message
*/
func (c *Consensus) phase0Round1BeforeWait(seq uint32) {
	msg := c.genProposalMsg(seq)
	c.toNet(msg)
}

/*
	The code for Phase 0, Round 1, after enough messages are arrived
*/
func (c *Consensus) phase0Round1AfterWait(seq uint32) (ConsensusObj, bool) {
	c.PanicTermNotMatched(seq)
	slot := seq % Conf.LenLedger
	if c.Ledger[slot].RecvProposalsMajT() >= Conf.MajorityPlusF {
		msg := c.genDecMsgType1(seq)
		c.toNet(msg)
		c.Ledger[slot].Round++
		return c.Ledger[slot].RecvProposalsMajV(), true
	} else if c.Ledger[slot].RecvProposalsMajT() >= Conf.Majority {
		c.Ledger[slot].SetMyBCMsgs(0, 2, 1) // Vote[0,2] = 1
	} else {
		c.Ledger[slot].SetMyBCMsgs(0, 2, 2) // Vote[0,2] = ?
	}
	c.Ledger[slot].Round++
	return ConsensusObj{}, false
}

/*
	The code for Phase 0, Round 2, before waiting for enough message
*/
func (c *Consensus) phase0Round2BeforeWait(seq uint32) {
	msg := c.genBinConMsg(seq, 0, 2)
	c.toNet(msg)
}

/*
	The code for Phase 0, Round 2, after enough messages are arrived
*/
func (c *Consensus) phase0Round2AfterWait(seq uint32) (ConsensusObj, bool) {
	c.PanicTermNotMatched(seq)
	slot := seq % Conf.LenLedger
	if c.Ledger[slot].RecvBCMsgsMajT(0, 2) >= Conf.FaultyPlusOne {
		m := c.findReturnValue(seq, 0, 2)
		msg := c.genDecMsgType2(seq, m)
		c.toNet(msg)
		c.Ledger[slot].Round++
		return m, true
	} else if c.Ledger[slot].RecvBCMsgsMajT(0, 2) >= 1 {
		c.Ledger[slot].SetMyBCMsgs(1, 1, c.Ledger[slot].RecvBCMsgsMajV(0, 2)) // State[1] = MajV(Vote[0])
	} else {
		c.Ledger[slot].SetMyBCMsgs(1, 1, 0) // State[1] = 0
	}
	c.Ledger[slot].IncrPhaseDecrRound()
	if c.Ledger[slot].Round != 1 {
		panic("here c.Ledger[slot].Round != 1")
	}
	return ConsensusObj{}, false

}

/*
	The code for Phase 1-N, Round 1, before waiting for enough message
*/
func (c *Consensus) phaseNRound1BeforeWait(seq uint32) {
	c.PanicTermNotMatched(seq)
	slot := seq % Conf.LenLedger
	pse := c.Ledger[slot].Phase
	msg := c.genBinConMsg(seq, pse, 1)
	c.toNet(msg)
}

/*
	The code for Phase 1-N, Round 1, after enough messages are arrived
*/
func (c *Consensus) phaseNRound1AfterWait(seq uint32) (ConsensusObj, bool) {
	c.PanicTermNotMatched(seq)
	slot := seq % Conf.LenLedger
	pse := c.Ledger[slot].Phase
	if c.Ledger[slot].RecvBCMsgsMajT(pse, 1) >= Conf.MajorityPlusF {
		m := c.findReturnValue(seq, pse, 1)
		msg := c.genDecMsgType2(seq, m)
		c.toNet(msg)
		c.Ledger[slot].Round++
		return m, true
	} else if c.Ledger[slot].RecvBCMsgsMajT(pse, 1) >= Conf.Majority {
		c.Ledger[slot].SetMyBCMsgs(pse, 2, c.Ledger[slot].RecvBCMsgsMajV(pse, 1)) // Vote[p] = MajV(State[p])
	} else {
		c.Ledger[slot].SetMyBCMsgs(pse, 2, 2) // Vote[p] = ?
	}
	c.Ledger[slot].Round++
	return ConsensusObj{}, false

}

/*
	The code for Phase 0, Round 2, before waiting for enough message
*/
func (c *Consensus) phaseNRound2BeforeWait(seq uint32) {
	c.PanicTermNotMatched(seq)
	slot := seq % Conf.LenLedger
	pse := c.Ledger[slot].Phase
	msg := c.genBinConMsg(seq, pse, 2)
	c.toNet(msg)

}

/*
	The code for Phase 0, Round 2, after enough messages are arrived
*/
func (c *Consensus) phaseNRound2AfterWait(seq uint32) (ConsensusObj, bool) {
	c.PanicTermNotMatched(seq)
	slot := seq % Conf.LenLedger
	pse := c.Ledger[slot].Phase
	randBit := c.CommonCoinFlip()
	if c.Ledger[slot].RecvBCMsgsMajT(pse, 2) >= Conf.FaultyPlusOne {
		m := c.findReturnValue(seq, pse, 2)
		msg := c.genDecMsgType2(seq, m)
		c.toNet(msg)
		c.Ledger[slot].Round++
		return m, true
	} else if c.Ledger[slot].RecvBCMsgsMajT(pse, 2) >= 1 {
		c.Ledger[slot].SetMyBCMsgs(pse+1, 1, c.Ledger[slot].RecvBCMsgsMajV(pse, 2)) // State[p+1] = MajV(Vote[p])
	} else {
		c.Ledger[slot].SetMyBCMsgs(pse+1, 1, randBit) // State[p+1] = randBit
	}
	c.Ledger[slot].IncrPhaseDecrRound()
	return ConsensusObj{}, false
}

/*
	sends a proposal request and waits for a respective reply
*/
func (c *Consensus) requestProposalAndWait(seq uint32) ConsensusObj {
	//fmt.Println(c.SvrId, "seq = ", seq, "requestProposalAndWait")
	msg := Msg{Phase: c.SvrId, Type: ProposalRequest, Value: seq}
	c.toNet(msg)
	for {
		msg := <-c.NetToConExecutor
		if msg.Type != ProposalReply {
			panic(fmt.Sprint("should not happen, msg.Type != ProposalReply"))
		}
		if msg.Value < seq {
			continue
		}
		//fmt.Println(c.SvrId, "seq = ", seq, "requestProposalAndWait done")
		return *msg.Obj
	}
}

/*
	The find return value function that follows Rabia's pseudo-code
*/
func (c *Consensus) findReturnValue(seq, pse, rod uint32) ConsensusObj {
	c.PanicTermNotMatched(seq)
	slot := seq % Conf.LenLedger
	if c.Ledger[slot].RecvBCMsgsMajV(pse, rod) == 1 {
		if c.Ledger[slot].RecvProposalsMajT() >= Conf.Majority {
			obj := c.Ledger[slot].RecvProposalsMajV()
			obj.SvrSeq = seq
			return obj
		} else {
			return c.requestProposalAndWait(seq)
		}
	} else {
		return ConsensusObj{IsNull: true, SvrSeq: seq}
	}
}

/*
	Sends a message to the network layer
*/
func (c *Consensus) toNet(msg Msg) {
	c.ConExecutorToNet <- msg
}

/*
	Sets my proposal and return true if there's a pending request, otherwise, return false
*/
func (c *Consensus) getRequest() bool {
	if obj, ok := c.QPop(); ok {
		if c.Discard[obj.GetIdSeq()] {
			delete(c.Discard, obj.GetIdSeq())
			return false
		} else {
			//c.SvrSeq += Conf.NConcurrency
			c.SvrSeq += 1
			c.UpdateTermIfNecessary(uint32(c.SvrSeq), true)
			slot := uint32(c.SvrSeq) % Conf.LenLedger
			c.Ledger[slot].SetMyProposal(obj)
			c.Ledger[slot].Round = 1
			c.ResetCommonCoin()
			return true
		}
	} else {
		return false
	}
}

/*
	Actions performed when a decision is reached
*/
func (c *Consensus) epilogue(seq uint32, dec ConsensusObj) {
	c.PanicTermNotMatched(seq)
	slot := seq % Conf.LenLedger

	c.Ledger[slot].Decision = dec
	c.Ledger[slot].IsDone = true

	if dec.IsNull {
		c.NullSlots++
		c.CurrConsecutiveNulls++
		c.putBackMyProposal(seq)

	} else {
		if c.CurrConsecutiveNulls > c.MaxConsecutiveNulls {
			c.MaxConsecutiveNulls = c.CurrConsecutiveNulls
			c.MaxConsecutiveNullsEndSeq = int(dec.SvrSeq)
		}
		c.CurrConsecutiveNulls = 0

		if !ProxySeqIdEqual(&dec, &c.Ledger[slot].MyProposal) {
			c.UnmatchedSlots++
			c.putBackMyProposal(seq)
			c.Discard[dec.GetIdSeq()] = true

		} else {
			c.NormalSlots++
		}

		// whether we did UnmatchedSlots++ or NormalSlots++, some client-requests are processed, so we do the following
		c.NumClientBatchedRequests += len(dec.CliIds)
	}

	/*
		Since this implementation is a mix of our old and new versions of pseudo-code, the variable currentRoundNum
		below may not equal to the number of rounds described in our new algorithm in our paper. See the package-level
		comments for more information.

		Now, NumOfRounds and NumOfRoundsDist store the new version of round numbers. Per1000RoundDist supports both
		versions -- use the UseOldVersion flag switch version.

		Anyhow, the conversion table is here:
			old version: 1 2 3 4 5 6 7 8 9 ... 18 19 20
			new version: 3 3 3 5 5 7 7 9 9 ... 19 19 21

		Note 1: currentRoundNum below is never 0 (because round numbers are never zero after the getRequest function call)

		Note 2: if Per1000RoundDist(old/new) is [0 7 16 977], it does NOT mean there are 7 2-round slots,
		instead, it means there are 7 1-round slots among the pass 1000 slots.
	*/
	currentRoundNum := c.Ledger[slot].Phase*2 + c.Ledger[slot].Round
	if currentRoundNum <= 3 {
		currentRoundNum = 3
	} else if currentRoundNum%2 == 0 {
		currentRoundNum += 1
	}
	c.TotalRounds += currentRoundNum
	//c.NumOfRounds[c.NumOfRoundsIdx] = currentRoundNum
	c.NumOfRoundsDist[currentRoundNum] += 1

	/*
		Below is the code for Per1000RoundDists
	*/
	//UseOldVersion := true
	//c.Per1000RoundDist[currentRoundNum] += 1
	//c.NumOfRoundsIdx++ // deprecated when NumOfRounds is deprecated
	//if c.NumOfRoundsIdx%1000 == 0 {
	//	if UseOldVersion {
	//		c.RoundDistLogger.Info().
	//			//Int("start", c.NumOfRoundsIdx-1000).
	//			//Int("end", c.NumOfRoundsIdx-1).
	//			Ints("RoundDist(old)", RemoveTrailingZeros(c.Per1000RoundDist)).Msg("")
	//		// If the array is [0 7 16 977], that means there are
	//		// 7 1-round slots, 16 2-round slots, and 977 3-round slots
	//
	//	} else {
	//		newVersion := make([]int, Conf.LenBlockArray*2+2) // why + 2: if old = 20, new = 21, then [21] is used
	//		for i := 0; i < len(c.Per1000RoundDist); i++ {
	//			if i <= 3 {
	//				newVersion[3] += c.Per1000RoundDist[i]
	//			} else if i%2 == 0 {
	//				newVersion[i+1] += c.Per1000RoundDist[i]
	//			} else {
	//				newVersion[i] += c.Per1000RoundDist[i]
	//			}
	//		}
	//		c.RoundDistLogger.Info().
	//			//Int("start", c.NumOfRoundsIdx-1000).
	//			//Int("end", c.NumOfRoundsIdx-1).
	//			Ints("RoundDist(new)", RemoveTrailingZeros(newVersion)).Msg("")
	//		// If the array is [0 0 0 958 0 38 0 4], that means there are
	//		// 958 3-round slots, 38 5-round slots, and 4 7-round slots
	//	}
	//	c.Per1000RoundDist = make([]int, Conf.LenBlockArray*2+1)
	//}
}

/*
	Put my proposal back to the request pending queue
*/
func (c *Consensus) putBackMyProposal(seq uint32) {
	c.PanicTermNotMatched(seq)
	slot := seq % Conf.LenLedger
	obj := c.Ledger[slot].MyProposal
	c.QPush(obj)
}

func (c *Consensus) logExitStatus() {
	c.TotalSlots = c.NormalSlots + c.UnmatchedSlots + c.NullSlots

	c.Logger.Warn().
		Str("status", "exit").
		Uint32("SvrId", c.SvrId).
		Uint32("InsId", c.InsId).
		Int("NormalSlots", c.NormalSlots).
		Int("UnmatchedSlots", c.UnmatchedSlots).
		Int("NullSlots", c.NullSlots).
		Int("TotalSlots", c.TotalSlots).
		Int("NumClientBatchedRequests", c.NumClientBatchedRequests).
		Int("NumClientUnbatchedRequests", c.NumClientBatchedRequests*Conf.ClientBatchSize).
		Uint32("TotalRounds", c.TotalRounds).
		Float64("avgNumOfRounds", float64(c.TotalRounds)/float64(c.TotalSlots)).
		Int("p95NumOfrounds", c.findRds(95)).
		Int("p99NumOfrounds", c.findRds(99)).
		Int("maxNumOfrounds", c.findRds(100)).
		Ints("roundsDistribution", RemoveTrailingZeros(c.NumOfRoundsDist)).
		Int("OlderThanTermMsg", c.OlderThanTermMsg).
		Int("MaxConsecutiveNulls", c.MaxConsecutiveNulls).
		Int("MaxConsecutiveNullsEndSeq", c.MaxConsecutiveNullsEndSeq).Msg("")
}
