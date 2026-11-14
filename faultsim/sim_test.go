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
package faultsim

import (
	"rabia/internal/message"
	"strings"
	"testing"
	"time"
)

func req(pro, seq uint32, cmd string) message.ConsensusObj {
	return message.ConsensusObj{ProId: pro, ProSeq: seq, CliIds: []uint32{pro}, CliSeqs: []uint32{seq}, Commands: []string{cmd}}
}

// cleanObservation is three replicas that all decided slot 0 = a (proposed by all three) and slot 1 = null.
func cleanObservation() observation {
	a, b := req(0, 0, "a"), req(1, 0, "b")
	null := message.ConsensusObj{IsNull: true}
	return observation{
		n: 3,
		decisions: []map[uint32]message.ConsensusObj{
			{0: a, 1: null}, {0: a, 1: null}, {0: a},
		},
		proposals: map[uint32]map[int]message.ConsensusObj{
			0: {0: a, 1: a, 2: a},
			1: {0: a, 1: b, 2: b},
		},
		requests: map[string]message.ConsensusObj{a.GetIdSeq(): a, b.GetIdSeq(): b},
	}
}

func expectOne(t *testing.T, name string, got []string, prefix string) {
	t.Helper()
	if len(got) != 1 || !strings.HasPrefix(got[0], prefix) {
		t.Errorf("%s: want one %q line, got %q", name, prefix, got)
	}
}

func TestCheckerOnHandMadeObservations(t *testing.T) {
	if v := checkSafety(cleanObservation()); len(v) != 0 {
		t.Fatalf("clean observation: %q", v)
	}

	o := cleanObservation()
	o.decisions[2][0] = req(1, 0, "b")
	o.proposals[0][2] = req(1, 0, "b")
	o.proposals[0][1] = req(1, 0, "b")
	// replicas 0,1 decided a (now proposed by one replica), replica 2 decided b (proposed by two)
	found := false
	for _, line := range checkSafety(o) {
		found = found || strings.HasPrefix(line, "agreement:")
	}
	if !found {
		t.Errorf("agreement: got %q", checkSafety(o))
	}

	o = cleanObservation()
	o.decisions[2][1] = req(1, 0, "b")
	o.proposals[1][0] = req(1, 0, "b")
	expectOne(t, "null-split", checkSafety(o), "null-split:")

	o = cleanObservation()
	ghost := req(7, 7, "ghost")
	for r := range o.decisions {
		o.decisions[r][0] = ghost
	}
	o.proposals[0] = map[int]message.ConsensusObj{0: ghost, 1: ghost, 2: ghost}
	expectOne(t, "not submitted", checkSafety(o), "validity:")

	o = cleanObservation()
	o.proposals[0] = map[int]message.ConsensusObj{}
	expectOne(t, "not proposed", checkSafety(o), "validity:")

	o = cleanObservation()
	o.proposals[0] = map[int]message.ConsensusObj{0: req(0, 0, "a")}
	expectOne(t, "minority", checkSafety(o), "majority:")

	o = cleanObservation()
	bad := req(0, 0, "tampered")
	for r := range o.decisions {
		o.decisions[r][0] = bad
	}
	expectOne(t, "integrity", checkSafety(o), "integrity:")

	o = cleanObservation()
	o.decisions[1][2] = req(0, 0, "a")
	expectOne(t, "duplicate", checkDuplicates(o), "duplicate:")
}

func runProfile(t *testing.T, profile string, n int, seed int64) Result {
	t.Helper()
	cfg, err := Profile(profile, n, seed)
	if err != nil {
		t.Fatal(err)
	}
	cfg.TraceDir = t.TempDir()
	return Run(cfg)
}

func TestFaultFreeRunsDecideEverything(t *testing.T) {
	for _, n := range []int{3, 5} {
		for seed := int64(1); seed <= 3; seed++ {
			r := runProfile(t, "delay", n, seed)
			if r.Outcome != "ok" || r.Required != r.Submitted || r.Submitted == 0 {
				t.Errorf("n=%d seed=%d: %+v", n, seed, r)
			}
		}
	}
}

// Every profile must keep the safety properties. Stalls are allowed here and only logged: see TestDecisionNotRelayed.
func TestProfilesKeepSafety(t *testing.T) {
	seeds := int64(10)
	if testing.Short() {
		seeds = 2
	}
	for _, p := range Profiles {
		for _, n := range []int{3, 5} {
			for seed := int64(1); seed <= seeds; seed++ {
				r := runProfile(t, p, n, seed)
				if len(r.Safety) > 0 || len(r.Panics) > 0 || r.Outcome == "wrapped" || r.Outcome == "hang" {
					t.Errorf("%s n=%d seed=%d: %s %q %q (trace %s)", p, n, seed, r.Outcome, r.Safety, r.Panics, r.TraceFile)
				} else if r.Outcome != "ok" {
					t.Logf("%s n=%d seed=%d: %s %q", p, n, seed, r.Outcome, r.Stalls)
				}
			}
		}
	}
}

// Each mutant breaks a quorum constant. If no run exposes it, the harness has no power. The pinned seeds are runs of
// the tail profile that "campaign run -mutants ..." found to violate safety. Runs replay deterministically, so each
// pinned seed should fail again; if one does not, the test searches further seeds before giving up.
func TestMutantsAreCaught(t *testing.T) {
	pinned := map[string]map[int]int64{
		"fast-path":    {3: 23, 5: 2},
		"low-majority": {3: 3, 5: 1},
		"weak-decide":  {3: 16, 5: 14},
	}
	for _, m := range Mutants {
		for _, n := range []int{3, 5} {
			seeds := []int64{pinned[m][n]}
			for s := int64(1); s <= 60; s++ {
				seeds = append(seeds, s)
			}
			caught := false
			for i, seed := range seeds {
				cfg, _ := Profile("tail", n, seed)
				cfg.Mutant = m
				if r := Run(cfg); len(r.Safety) > 0 {
					caught = true
					if i > 0 {
						t.Logf("%s n=%d: pinned seed %d did not replay; caught at seed %d", m, n, pinned[m][n], seed)
					}
					t.Logf("%s n=%d seed %d: %s", m, n, seed, r.Safety[0])
					break
				}
			}
			if !caught {
				t.Errorf("%s n=%d: no safety violation", m, n)
			}
		}
	}
}

// decisionNotRelayed is a three-replica schedule with one crash (f = 1):
//
//  1. Replica 1 hears of request 0-0 late (its client request is delayed to tick 200).
//  2. Replicas 0 and 2 propose 0-0 and vote 1. Replica 2's vote to replica 0 is slow.
//  3. Replica 2 has two votes of 1 (its own and replica 0's), decides 0-0 and broadcasts a Decision. The copy to
//     replica 1 arrives at once; the copy to replica 0 is queued behind the slow vote.
//  4. Replica 2 crashes at tick 150 and loses both messages to replica 0.
//  5. When replica 1 reaches slot 0 it already has the Decision, so it decides without voting and does not pass the
//     Decision on (wait() in executor.go, line 53 of Algorithm 4 in docs/verbose-algorithm.pdf).
//  6. Replica 0 waits for a second phase-0 vote that no live replica will send. Request 1-0 then stalls replica 1 in
//     slot 1, which needs a proposal from replica 0.
func decisionNotRelayed(crash bool) Config {
	cfg := Config{Name: "scripted", N: 3, F: 1, Seed: 1, MinDelay: 1, MaxDelay: 1, MaxTicks: 10000,
		Quiet: 200 * time.Microsecond, LenLedger: 64, RequestAt: []int{0, 250}, LoseAll: true,
		Delay: func(from, to int, m message.Msg, tick int) (int, bool) {
			switch {
			case m.Type == message.ClientRequest && from == 0 && to == 1:
				return 200, true
			case m.Type == message.Vote && from == 2 && to == 0:
				return 300, true
			}
			return 1, true
		}}
	if crash {
		cfg.CrashAt = map[int]int{2: 150}
	}
	return cfg
}

func TestDecisionNotRelayed(t *testing.T) {
	control := Run(decisionNotRelayed(false))
	if control.Outcome != "ok" {
		t.Fatalf("without the crash the schedule should finish: %+v", control)
	}
	r := Run(decisionNotRelayed(true))
	if len(r.Safety) > 0 || len(r.Panics) > 0 {
		t.Fatalf("unexpected: %+v", r)
	}
	if r.Outcome != "stall" || len(r.Stalls) != 2 {
		t.Fatalf("expected replicas 0 and 1 to stall (a fix that relays decisions would make this test fail): %+v", r)
	}
	for _, s := range r.Stalls {
		t.Log(s)
	}
}

// poisonRequest is a three-replica schedule with one crash (f = 1):
//
//  1. Each replica's proxy sends one request at tick 0. Replica 2's request 2-0 reaches replicas 0 and 2, but its
//     copy to replica 1 is slow.
//  2. Replica 2 crashes at tick 10 and its copy to replica 1 is lost. Replicas 0 and 1 decide 0-0 and 1-0.
//  3. 2-0 now heads replica 0's pending queue. Replica 1 never received it and proposes the next request, 0-1.
//     Neither value has a majority of three, so the slot is null, both replicas put their proposal back, and the
//     next slot repeats this.
func poisonRequest(crash bool) Config {
	cfg := Config{Name: "scripted", N: 3, F: 1, Seed: 1, MinDelay: 1, MaxDelay: 1, MaxTicks: 20000,
		Quiet: 200 * time.Microsecond, LenLedger: 256, RequestAt: []int{0, 0, 0, 300, 301}, LoseAll: true,
		Delay: func(from, to int, m message.Msg, tick int) (int, bool) {
			if m.Type == message.ClientRequest && from == 2 && to == 1 {
				return 1000, true
			}
			return 1, true
		}}
	if crash {
		cfg.CrashAt = map[int]int{2: 10}
	}
	return cfg
}

func TestPoisonRequest(t *testing.T) {
	control := Run(poisonRequest(false))
	if control.Outcome != "ok" {
		t.Fatalf("without the crash the schedule should finish: %+v", control)
	}
	r := Run(poisonRequest(true))
	if len(r.Safety) > 0 || len(r.Panics) > 0 {
		t.Fatalf("unexpected: %+v", r)
	}
	tagged := map[string]bool{}
	for _, tag := range r.Liveness {
		tagged[tag] = true
	}
	if r.Outcome != "timeout" || !tagged["poison-request"] || !tagged["null-livelock"] {
		t.Fatalf("expected a null livelock behind request 2-0: %+v", r)
	}
	t.Logf("%d null slots; %q", r.NullSlots, r.Stalls)
}

// noLiveWitness is a three-replica schedule with one crash (f = 1). Some links are slow, and the first three slots
// end null. Then, in slot 3:
//
//  1. Replicas 0 and 1 propose 0-0, and replica 2 proposes 2-0. The copies of 0-0 between replicas 0 and 1 are slow.
//     Each of them counts its own 0-0 and replica 2's 2-0, and stops counting at n-f = 2 proposals.
//  2. Only replica 2 counts two copies of 0-0. It is the only replica that votes 1 in phase 0.
//  3. Every replica decides 1, meaning a value. Replica 2 knows the value is 0-0. Replicas 0 and 1 have no majority
//     value, so they send a ProposalRequest, which only a replica that counted a majority answers.
//  4. Replica 2 crashes at tick 250, losing its Decision and the requests still in flight to it. Replicas 0 and 1
//     wait for a ProposalReply forever, although both of them proposed 0-0.
func noLiveWitness(crash bool) Config {
	cfg := Config{Name: "scripted", N: 3, F: 1, Seed: 1, MinDelay: 1, MaxDelay: 1, MaxTicks: 20000,
		Quiet: 200 * time.Microsecond, LenLedger: 64, RequestAt: []int{0, 500, 0}, LoseAll: true,
		Delay: func(from, to int, m message.Msg, tick int) (int, bool) {
			switch {
			case m.Type == message.ClientRequest && from == 0 && to == 2:
				return 200, true
			case m.Type == message.ClientRequest && from == 2 && to != 2:
				return 5, true
			case m.Type == message.Proposal && from == 2 && to == 2:
				return 300, true
			case (m.Type == message.Proposal || m.Type == message.Vote && m.Phase == 0) && from+to == 1:
				return 50, true
			case m.Type == message.Decision && from == 2:
				return 300, true
			case m.Type == message.ProposalRequest && to == 2:
				return 300, true
			}
			return 1, true
		}}
	if crash {
		cfg.CrashAt = map[int]int{2: 250}
	}
	return cfg
}

func TestNoLiveWitness(t *testing.T) {
	control := Run(noLiveWitness(false))
	if control.Outcome != "ok" {
		t.Fatalf("without the crash the schedule should finish: %+v", control)
	}
	r := Run(noLiveWitness(true))
	if len(r.Safety) > 0 || len(r.Panics) > 0 {
		t.Fatalf("unexpected: %+v", r)
	}
	if r.Outcome != "stall" || len(r.Liveness) != 1 || r.Liveness[0] != "proposal-reply" {
		t.Fatalf("expected replicas 0 and 1 to wait for a ProposalReply: %+v", r)
	}
	for _, s := range r.Stalls {
		t.Log(s)
	}
}

func TestReplayIsStable(t *testing.T) {
	for _, p := range []string{"delay", "mixed"} {
		a, b := runProfile(t, p, 5, 7), runProfile(t, p, 5, 7)
		if a.TraceHash != b.TraceHash || a.Outcome != b.Outcome {
			t.Errorf("%s: seed 7 replayed to a different run: %s %s vs %s %s", p, a.Outcome, a.TraceHash, b.Outcome, b.TraceHash)
		}
	}
}
