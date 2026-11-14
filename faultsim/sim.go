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
	Package faultsim runs n unmodified Rabia consensus instances (the Executor and MsgHandler routines of
	roles/server/layers/consensus) in one process, connects them with an in-memory network that injects faults from a
	seeded schedule, and checks every decided slot against the safety properties proved in proofs/ivy/weak_mvc.ivy and
	proofs/coq/weak_mvc.v. See README.md in this folder for the fault model and what a seed does and does not fix.
*/
package faultsim

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"rabia/internal/config"
	"rabia/internal/ledger"
	"rabia/internal/message"
	"rabia/roles/server/layers/consensus"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Profiles are the named fault mixes accepted by Profile.
var Profiles = []string{"delay", "tail", "partition", "crash", "mixed"}

// Mutants are negative controls. Each one breaks a quorum constant in config.Conf for every replica, so a run with
// a mutant is a run of a wrong implementation, and the checker should report safety violations for some seeds.
//   - fast-path: a replica decides in phase 0 when a majority of its n-f proposals match, in place of n/2+f+1
//   - low-majority: a value counts as a majority with n/2 proposals in place of n/2+1
//   - weak-decide: a replica decides on a single matching vote in place of f+1
var Mutants = []string{"fast-path", "low-majority", "weak-decide"}

const (
	sentinelProId = math.MaxUint32 // marks the fake ProposalReply used to unblock an executor at shutdown
	chanCap       = 4096
)

type Config struct {
	Name string // the profile name, copied into the Result
	N    int    // number of replicas
	F    int    // number of crash failures the replicas tolerate (sets n-f, f+1, ...)
	Seed int64  // fixes the request schedule, message delays, partitions, crashes and dropped suffixes

	Requests   int // number of client requests, sent round-robin through the replicas' proxies
	RequestGap int // average ticks between two requests

	MinDelay int     // smallest message delay in ticks
	MaxDelay int     // largest regular message delay in ticks
	TailProb float64 // probability that a message gets an extra heavy-tailed delay
	TailMax  int     // largest extra delay in ticks

	Partitions   int // number of temporary partitions; messages across a partition wait until it heals
	PartitionMax int // longest partition in ticks
	Crashes      int // number of crash-stop failures, at most F
	FaultWindow  int // partitions start and crashes happen at a tick in [0, FaultWindow)

	MaxTicks  int           // a run that has not gone quiet by this tick ends as a timeout
	MaxSlots  int           // a run whose replicas propose for a slot past this ends as a timeout (0: 10*Requests+50)
	Quiet     time.Duration // real-time window used to decide that the replicas have reacted to a tick
	LenLedger uint32        // ledger length; a run that reaches it ends as "wrapped"
	Mutant    string        // "" or one of Mutants
	TraceDir  string        // where the traces of runs that are not ok are written ("" = nowhere)

	// Scripted runs, used by tests. When set, these replace the random parts of the schedule.
	CrashAt   map[int]int `json:",omitempty"` // replica -> tick of its crash, in place of Crashes
	LoseAll   bool        `json:",omitempty"` // a crash loses everything the replica still has in flight
	RequestAt []int       `json:",omitempty"` // tick of each request (proxy = index % N), in place of RequestGap
	// Delay gives the delay of one message in ticks, or false to use the random delay.
	Delay func(from, to int, m message.Msg, tick int) (int, bool) `json:"-"`
}

/*
Profile returns the configuration of the fault mix called name, for n replicas and f = (n-1)/2.
*/
func Profile(name string, n int, seed int64) (Config, error) {
	c := Config{Name: name, N: n, F: (n - 1) / 2, Seed: seed, Requests: 10, RequestGap: 4, MinDelay: 1, MaxDelay: 10,
		MaxTicks: 100000, Quiet: 200 * time.Microsecond, LenLedger: 1024}
	switch name {
	case "delay":
	case "tail":
		c.MaxDelay, c.TailProb, c.TailMax = 5, 0.1, 200
	case "partition":
		c.Partitions, c.PartitionMax, c.FaultWindow = 2, 150, 300
	case "crash":
		c.Crashes, c.FaultWindow = c.F, 300
	case "mixed":
		c.TailProb, c.TailMax = 0.05, 200
		c.Partitions, c.PartitionMax = 2, 150
		c.Crashes, c.FaultWindow = c.F, 300
	default:
		return c, fmt.Errorf("unknown profile %q", name)
	}
	return c, nil
}

/*
Result describes one run. Outcome is the most serious category that applies, in this order: wrapped (the ledger
ring wrapped, so the checks are inconclusive), safety, panic, wedged (a replica stopped taking messages), timeout,
hang (the replicas did not exit at shutdown), stall (a live replica did not decide every required request),
duplicate, ok.
*/
type Result struct {
	Profile string `json:"profile,omitempty"`
	N       int    `json:"n"`
	F       int    `json:"f"`
	Seed    int64  `json:"seed"`
	Mutant  string `json:"mutant,omitempty"`

	Outcome    string   `json:"outcome"`
	Safety     []string `json:"safety,omitempty"`
	Panics     []string `json:"panics,omitempty"`
	Stalls     []string `json:"stalls,omitempty"`
	Duplicates []string `json:"duplicates,omitempty"`
	// Liveness tags the stalls and timeouts: decided-elsewhere (a live replica waits in a slot another live replica
	// has decided), poison-request (a live replica holds a request another live replica never received),
	// proposal-reply (a live replica waits for a ProposalReply), null-livelock (a timeout with more null slots than
	// ten times the requests)
	Liveness []string `json:"liveness,omitempty"`

	Ticks     int   `json:"ticks"`
	Delivered int   `json:"delivered"`
	Dropped   int   `json:"dropped"`
	Crashed   []int `json:"crashed,omitempty"`
	Submitted int   `json:"submitted"` // requests a proxy started to broadcast
	Required  int   `json:"required"`  // requests whose proxy never crashed; every live replica must decide them

	Slots         int    `json:"slots"`     // highest number of decided slots at a live replica
	NullSlots     int    `json:"nullSlots"` // slots some replica decided null
	MaxPhase      int    `json:"maxPhase"`  // highest phase any replica reached in any slot
	FastPathSlots int    `json:"fastPath"`  // (replica, slot) pairs whose phase-0 tally reached MajorityPlusF
	TraceHash     string `json:"traceHash"` // hash of the delivered message sequence, for replay comparisons
	WallMs        int64  `json:"wallMs"`    // real time of the run
	TraceFile     string `json:"trace,omitempty"`
}

type node struct {
	id        int
	c         *consensus.Consensus
	led       ledger.Ledger
	toHandler chan message.Msg // network -> MsgHandler
	toExec    chan message.Msg // network -> Executor (ProposalReply)
	fromExec  chan message.Msg // Executor -> network (broadcasts)
	fromHdl   chan message.Msg // MsgHandler -> network (ProposalReply)
	done      chan struct{}    // closed when the replica crashes or the run ends
	closed    bool             // done is closed (router only)
	lastTick  atomic.Int64     // tick of the latest delivery to this replica: the cause of what it sends next
	execGID   atomic.Int64     // goroutine ids of the two consensus routines, for settled()
	hdlGID    atomic.Int64
	dead      atomic.Bool // a routine of this replica panicked
	execDone  atomic.Bool // the Executor routine has returned
}

type envelope struct {
	from, to  int
	toExec    bool
	sentAt    int
	deliverAt int
	tie       int64
	linkSeq   uint64
	msg       message.Msg
}

type outItem struct {
	from    int
	handler bool
	sentAt  int
	seq     uint64
	msg     message.Msg
}

type partition struct {
	start, end int
	side       []bool
}

type injection struct {
	tick, proxy int
	obj         message.ConsensusObj
}

type traceEvent struct {
	Tick   int    `json:"t"`
	Ev     string `json:"ev"`
	From   int    `json:"from"`
	To     int    `json:"to"`
	Type   string `json:"type,omitempty"`
	Phase  uint32 `json:"phase,omitempty"`
	Value  uint32 `json:"value,omitempty"`
	Seq    uint32 `json:"seq,omitempty"`
	ProId  uint32 `json:"proId,omitempty"`
	ProSeq uint32 `json:"proSeq,omitempty"`
	IsNull bool   `json:"isNull,omitempty"`
	Note   string `json:"note,omitempty"`
}

type sim struct {
	cfg   Config
	nodes []*node
	wg    sync.WaitGroup // the consensus routines
	fwdWg sync.WaitGroup // the forwarders
	stop  chan struct{}  // stops the forwarders

	mu      sync.Mutex // guards outbox and outSeq
	outbox  []outItem
	outSeq  [][2]uint64
	sends   atomic.Int64
	fwdGIDs []atomic.Int64 // goroutine ids of the forwarders
	stack   []byte         // buffer for runtime.Stack

	panicMu  sync.Mutex
	panics   []string
	stacks   []string
	stopping atomic.Bool

	pending   []*envelope
	maxSlot   int // highest slot seen in any consensus message
	linkSeq   [][]uint64
	lastAt    [][]int
	crashed   []bool
	crashes   map[int][]int // tick -> replicas that crash at that tick
	willCrash []bool
	parts     []partition
	inject    []injection
	nextInj   int

	proposals map[uint32]map[int]message.ConsensusObj
	requests  map[string]message.ConsensusObj
	required  map[string]bool
	submitted int

	got      []map[string]bool // replica -> client requests delivered to it
	askedFor []int             // replica -> slot of its latest ProposalRequest, or -1
	answered []bool            // replica -> a ProposalReply for that slot reached it

	trace     []traceEvent
	hash      uint64
	delivered int
	dropped   int
}

/*
Run executes one seeded run and checks it. Runs share config.Conf, so they must not overlap in one process.
*/
func Run(cfg Config) Result {
	res, _ := run(cfg)
	return res
}

func run(cfg Config) (Result, *sim) {
	start := time.Now()
	if cfg.Crashes > cfg.F {
		panic("faultsim: Crashes must not exceed F")
	}
	if cfg.MaxSlots == 0 {
		cfg.MaxSlots = 10*max(cfg.Requests, len(cfg.RequestAt)) + 50
	}
	s := newSim(cfg)
	dir, err := os.MkdirTemp("", "faultsim-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	s.setConf(dir)
	s.start()
	ticks, status := s.loop()
	hung := !s.shutdown()

	res := s.check()
	res.Ticks = ticks
	res.TraceHash = fmt.Sprintf("%016x", s.hash)
	switch {
	case res.Outcome == "wrapped" || res.Outcome == "safety" || res.Outcome == "panic":
	case status != "":
		res.Outcome = status
	case hung:
		res.Outcome = "hang"
	}
	res.WallMs = time.Since(start).Milliseconds()
	if res.Outcome != "ok" && cfg.TraceDir != "" {
		res.TraceFile = s.writeTrace(res)
	}
	return res, s
}

func newSim(cfg Config) *sim {
	n := cfg.N
	s := &sim{
		cfg:       cfg,
		stop:      make(chan struct{}),
		outSeq:    make([][2]uint64, n),
		fwdGIDs:   make([]atomic.Int64, 2*n),
		stack:     make([]byte, 1<<16),
		crashed:   make([]bool, n),
		crashes:   map[int][]int{},
		willCrash: make([]bool, n),
		proposals: map[uint32]map[int]message.ConsensusObj{},
		requests:  map[string]message.ConsensusObj{},
		required:  map[string]bool{},
		hash:      fnv.New64a().Sum64(),
	}
	s.got = make([]map[string]bool, n)
	s.askedFor = make([]int, n)
	s.answered = make([]bool, n)
	for i := range s.got {
		s.got[i] = map[string]bool{}
		s.askedFor[i] = -1
	}
	s.linkSeq = make([][]uint64, n)
	s.lastAt = make([][]int, n)
	for i := range s.linkSeq {
		s.linkSeq[i] = make([]uint64, n)
		s.lastAt[i] = make([]int, n)
	}

	// The schedule (crashes, partitions, requests) comes from its own stream, so changing how many delays a run draws
	// does not move the faults.
	sched := rand.New(rand.NewSource(cfg.Seed * 2))
	if cfg.CrashAt != nil {
		for r, t := range cfg.CrashAt {
			s.crashes[t] = append(s.crashes[t], r)
			s.willCrash[r] = true
		}
		for _, rs := range s.crashes {
			sort.Ints(rs)
		}
	} else {
		for _, r := range sched.Perm(n)[:cfg.Crashes] {
			t := sched.Intn(max(cfg.FaultWindow, 1))
			s.crashes[t] = append(s.crashes[t], r)
			s.willCrash[r] = true
		}
	}
	for k := 0; k < cfg.Partitions; k++ {
		p := partition{start: sched.Intn(max(cfg.FaultWindow, 1)), side: make([]bool, n)}
		p.end = p.start + 1 + sched.Intn(max(cfg.PartitionMax, 1))
		// a random split with both sides non-empty
		cut := 1 + sched.Intn(n-1)
		for _, r := range sched.Perm(n)[:cut] {
			p.side[r] = true
		}
		s.parts = append(s.parts, p)
	}
	proSeq := make([]uint32, n)
	requests := cfg.Requests
	if cfg.RequestAt != nil {
		requests = len(cfg.RequestAt)
	}
	for k := 0; k < requests; k++ {
		proxy := k % n
		t := k*cfg.RequestGap + sched.Intn(max(cfg.RequestGap, 1))
		if cfg.RequestAt != nil {
			t = cfg.RequestAt[k]
		}
		obj := message.ConsensusObj{ProId: uint32(proxy), ProSeq: proSeq[proxy], CliIds: []uint32{uint32(k)},
			CliSeqs: []uint32{uint32(k)}, Commands: []string{fmt.Sprintf("0k%07dv%07d", k, k)}}
		proSeq[proxy]++
		s.inject = append(s.inject, injection{tick: t, proxy: proxy, obj: obj})
	}
	sort.SliceStable(s.inject, func(i, j int) bool { return s.inject[i].tick < s.inject[j].tick })
	return s
}

func (s *sim) setConf(dir string) {
	c := &config.Conf
	*c = config.Config{}
	c.ProjectFolder = dir
	c.LogLevel = "warn"
	c.NServers = s.cfg.N
	c.NFaulty = s.cfg.F
	c.NClients = 1
	c.NConcurrency = 1
	c.ClientBatchSize = 1
	c.ProxyBatchSize = 1
	c.CalcConstants()
	c.LenLedger = s.cfg.LenLedger
	switch s.cfg.Mutant {
	case "":
	case "fast-path":
		c.MajorityPlusF = c.Majority
	case "low-majority":
		c.Majority = s.cfg.N / 2
		c.MajorityPlusF = s.cfg.N/2 + s.cfg.F
	case "weak-decide":
		c.FaultyPlusOne = 1
	default:
		panic(fmt.Sprintf("faultsim: unknown mutant %q", s.cfg.Mutant))
	}
}

func (s *sim) start() {
	for i := 0; i < s.cfg.N; i++ {
		nd := &node{id: i,
			done:      make(chan struct{}),
			toHandler: make(chan message.Msg, chanCap),
			toExec:    make(chan message.Msg, chanCap),
			fromExec:  make(chan message.Msg, chanCap),
			fromHdl:   make(chan message.Msg, chanCap),
		}
		nd.led = make(ledger.Ledger, config.Conf.LenLedger)
		for k := range nd.led {
			nd.led[k] = &ledger.Slot{}
			nd.led[k].Reset()
		}
		nd.c = consensus.ConsensusInit(uint32(i), 0, nd.done, &s.wg, nd.toHandler, nd.fromHdl, nd.toExec, nd.fromExec, nd.led)
		s.nodes = append(s.nodes, nd)
	}
	for _, nd := range s.nodes {
		s.fwdWg.Add(2)
		go s.forward(nd.id, nd.fromExec, false, &s.fwdGIDs[2*nd.id])
		go s.forward(nd.id, nd.fromHdl, true, &s.fwdGIDs[2*nd.id+1])
	}
	for _, nd := range s.nodes {
		s.wg.Add(2)
		go s.guard(nd, "executor", func() { nd.c.Executor(); nd.execDone.Store(true) })
		go s.guard(nd, "handler", nd.c.MsgHandler)
	}
}

/*
guard runs a consensus routine and records a panic in place of crashing the process. A replica whose routine
panicked is treated as crashed from the next tick on.
*/
func (s *sim) guard(nd *node, role string, f func()) {
	if role == "executor" {
		nd.execGID.Store(goid())
	} else {
		nd.hdlGID.Store(goid())
	}
	defer func() {
		if r := recover(); r != nil {
			if role == "executor" {
				nd.execDone.Store(true)
			}
			stack := string(debug.Stack())
			msg := fmt.Sprintf("replica %d %s: %v at %s", nd.id, role, r, panicSite(stack))
			if s.stopping.Load() {
				msg = "after shutdown: " + msg
			}
			s.panicMu.Lock()
			s.panics = append(s.panics, msg)
			s.stacks = append(s.stacks, stack)
			s.panicMu.Unlock()
			nd.dead.Store(true)
		}
	}()
	f()
}

// panicSite returns the first Rabia frame below the panic call, such as "consensus.(*Consensus).wait executor.go:190".
func panicSite(stack string) string {
	lines := strings.Split(stack, "\n")
	seenPanic := false
	for i := 0; i+1 < len(lines); i++ {
		l := lines[i]
		if strings.HasPrefix(l, "panic(") {
			seenPanic = true
			continue
		}
		if seenPanic && strings.HasPrefix(l, "rabia/") && !strings.HasPrefix(l, "rabia/faultsim") {
			fn := l
			if k := strings.LastIndex(fn, "("); k > 0 {
				fn = fn[:k]
			}
			fn = fn[strings.LastIndex(fn, "/")+1:]
			loc := strings.TrimSpace(lines[i+1])
			if k := strings.LastIndex(loc, " +0x"); k > 0 {
				loc = loc[:k]
			}
			return fn + " " + filepath.Base(loc)
		}
	}
	return "unknown"
}

func (s *sim) forward(from int, ch chan message.Msg, handler bool, gid *atomic.Int64) {
	gid.Store(goid())
	defer s.fwdWg.Done()
	k := 0
	if handler {
		k = 1
	}
	for {
		select {
		case m := <-ch:
			s.mu.Lock()
			s.outbox = append(s.outbox, outItem{from: from, handler: handler, sentAt: int(s.nodes[from].lastTick.Load()),
				seq: s.outSeq[from][k], msg: m})
			s.outSeq[from][k]++
			s.mu.Unlock()
			s.sends.Add(1)
		case <-s.stop:
			return
		}
	}
}

/*
loop advances virtual time. At each tick it applies the scheduled crashes and client requests, delivers at most one
due message to each live replica, waits until the replicas have reacted, and gives every message they sent a
delivery tick. It returns the last tick and "" when the network is empty and nothing more is scheduled, or
"wedged" or "timeout".
*/
func (s *sim) loop() (int, string) {
	tick := 0
	for {
		if tick > s.cfg.MaxTicks || s.maxSlot > s.cfg.MaxSlots {
			return tick, "timeout"
		}
		for _, nd := range s.nodes {
			if nd.dead.Load() && !s.crashed[nd.id] {
				s.crash(nd.id, tick, "panic")
			}
		}
		for _, r := range s.crashes[tick] {
			s.crash(r, tick, "scheduled")
		}
		for s.nextInj < len(s.inject) && s.inject[s.nextInj].tick <= tick {
			in := s.inject[s.nextInj]
			s.nextInj++
			if s.crashed[in.proxy] {
				continue
			}
			s.submitted++
			id := in.obj.GetIdSeq()
			s.requests[id] = in.obj
			if !s.willCrash[in.proxy] {
				s.required[id] = true
			}
			s.record(traceEvent{Tick: tick, Ev: "inject", From: in.proxy, To: -1, ProId: in.obj.ProId, ProSeq: in.obj.ProSeq})
			for j := 0; j < s.cfg.N; j++ {
				s.send(in.proxy, j, false, message.Msg{Type: message.ClientRequest, Obj: copyObj(&in.obj)}, tick)
			}
		}
		if n, ok := s.deliver(tick); !ok {
			return tick, "wedged"
		} else if n > 0 && !s.waitQuiet() {
			return tick, "wedged"
		}
		s.flush(tick)

		next, more := s.nextTick(tick)
		if !more {
			// Nothing in flight and nothing scheduled: give the replicas one more chance to send, then stop.
			if !s.waitQuiet() {
				return tick, "wedged"
			}
			s.flush(tick)
			if len(s.pending) == 0 {
				return tick, ""
			}
			next = tick + 1
		}
		tick = next
	}
}

func (s *sim) nextTick(tick int) (int, bool) {
	next := math.MaxInt
	for _, e := range s.pending {
		next = min(next, max(e.deliverAt, tick+1))
	}
	for t := range s.crashes {
		if t > tick {
			next = min(next, t)
		}
	}
	if s.nextInj < len(s.inject) {
		next = min(next, max(s.inject[s.nextInj].tick, tick+1))
	}
	return next, next != math.MaxInt
}

/*
send gives a message a delivery tick: a random delay, held back while a partition separates the two replicas, and
never before an earlier message on the same link (links are FIFO, as TCP is, and the executor relies on it).
*/
func (s *sim) send(from, to int, toExec bool, m message.Msg, tick int) {
	c := s.cfg
	seq := s.linkSeq[from][to]
	d := c.MinDelay + int(s.rnd(1, from, to, seq)%uint64(c.MaxDelay-c.MinDelay+1))
	if c.TailProb > 0 && float64(s.rnd(2, from, to, seq)>>11)/(1<<53) < c.TailProb {
		d += 1 + int(s.rnd(3, from, to, seq)%uint64(max(c.TailMax, 1)))
	}
	if c.Delay != nil {
		if sd, ok := c.Delay(from, to, m, tick); ok {
			d = sd
		}
	}
	at := tick + d
	for changed := true; changed; {
		changed = false
		for _, p := range s.parts {
			if p.side[from] != p.side[to] && tick < p.end && at >= p.start && at < p.end {
				at, changed = p.end, true
			}
		}
	}
	if at <= s.lastAt[from][to] {
		at = s.lastAt[from][to] + 1
	}
	e := &envelope{from: from, to: to, toExec: toExec, sentAt: tick, deliverAt: at, tie: int64(s.rnd(4, from, to, seq) >> 1),
		linkSeq: seq, msg: m}
	if s.crashed[to] {
		s.drop(e, tick, "receiver crashed")
		return
	}
	s.lastAt[from][to] = at
	s.linkSeq[from][to]++
	s.pending = append(s.pending, e)
}

// rnd gives the random number with the given purpose for message seq on the link from -> to. Deriving it from the
// seed and the message's place on its link, in place of drawing from one shared stream, keeps a message's delay the
// same when goroutine timing changes which tick a send is noticed in.
func (s *sim) rnd(purpose, from, to int, seq uint64) uint64 {
	x := uint64(s.cfg.Seed)
	for _, v := range []uint64{uint64(purpose), uint64(from), uint64(to), seq} {
		x = splitmix(x ^ v)
	}
	return x
}

func splitmix(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}

// deliver hands each live replica its earliest due message. It returns false if a replica's input is full.
func (s *sim) deliver(tick int) (int, bool) {
	best := make([]*envelope, s.cfg.N)
	for _, e := range s.pending {
		if e.deliverAt > tick {
			continue
		}
		if b := best[e.to]; b == nil || e.deliverAt < b.deliverAt || (e.deliverAt == b.deliverAt && e.tie < b.tie) {
			best[e.to] = e
		}
	}
	n := 0
	for _, e := range best {
		if e != nil {
			n++
		}
	}
	if n == 0 {
		return 0, true
	}
	kept := s.pending[:0]
	for _, e := range s.pending {
		if best[e.to] != e {
			kept = append(kept, e)
		}
	}
	for i := len(kept); i < len(s.pending); i++ {
		s.pending[i] = nil
	}
	s.pending = kept
	for _, e := range best {
		if e == nil {
			continue
		}
		nd := s.nodes[e.to]
		ch := nd.toHandler
		if e.toExec {
			ch = nd.toExec
		}
		nd.lastTick.Store(int64(tick))
		select {
		case ch <- e.msg:
		default:
			return n, false
		}
		s.delivered++
		switch e.msg.Type {
		case message.ClientRequest:
			s.got[e.to][e.msg.Obj.GetIdSeq()] = true
		case message.ProposalReply:
			if int(e.msg.Value) == s.askedFor[e.to] {
				s.answered[e.to] = true
			}
		}
		ev := eventOf(tick, "deliver", e)
		s.record(ev)
		s.hashEvent(ev)
	}
	return n, true
}

/*
waitQuiet waits until no replica has sent anything for two quiet windows in a row and every input and output
channel is empty. It returns false if that does not happen within two seconds, which means a replica stopped
taking messages.
*/
func (s *sim) waitQuiet() bool {
	deadline := time.Now().Add(5 * time.Second)
	last, lastSeq := s.sends.Load(), s.svrSeqs()
	for stable := 0; stable < 2; {
		time.Sleep(s.cfg.Quiet)
		cur, seq := s.sends.Load(), s.svrSeqs()
		if cur == last && seq == lastSeq && s.channelsEmpty() && s.settled() {
			stable++
		} else {
			stable, last, lastSeq = 0, cur, seq
		}
		if time.Now().After(deadline) {
			return false
		}
	}
	return true
}

// goid returns the id of the calling goroutine, from the first line of its stack ("goroutine 12 [running]:").
func goid() int64 {
	var b [64]byte
	f := strings.Fields(string(b[:runtime.Stack(b[:], false)]))
	id, _ := strconv.ParseInt(f[1], 10, 64)
	return id
}

/*
settled reports whether every routine of every live replica, and every forwarder, is waiting for input. It reads
one snapshot of all goroutines (runtime.Stack stops the world while it writes it):

  - a MsgHandler must be parked in its select (an empty input channel is checked by channelsEmpty);
  - an Executor must be parked (in wait or requestProposalAndWait) or busy-waiting in getRequest with an empty pending
    queue, with nothing but the Executor loop, getRequest and QPop of the consensus package on its stack;
  - a forwarder must be parked in its select.

Real-time quiet windows were not enough: a replica's reaction to a message sometimes took tens of milliseconds,
mostly while its MsgHandler waited for the pending-queue lock that the busy-waiting Executor keeps taking.
*/
func (s *sim) settled() bool {
	for {
		n := runtime.Stack(s.stack, true)
		if n < len(s.stack) {
			s.stack = s.stack[:cap(s.stack)]
			return s.settledIn(string(s.stack[:n]))
		}
		s.stack = make([]byte, 2*len(s.stack))
	}
}

func (s *sim) settledIn(dump string) bool {
	blocks := map[int64]string{}
	for _, b := range strings.Split(dump, "\n\n") {
		var id int64
		if _, err := fmt.Sscanf(b, "goroutine %d ", &id); err == nil {
			blocks[id] = b
		}
	}
	state := func(b string) string {
		i, j := strings.Index(b, "["), strings.Index(b, "]")
		if i < 0 || j < i {
			return ""
		}
		return b[i+1 : j]
	}
	parked := func(b string) bool {
		st := state(b)
		return strings.HasPrefix(st, "select") || strings.HasPrefix(st, "chan receive")
	}
	for i := range s.fwdGIDs {
		if b, ok := blocks[s.fwdGIDs[i].Load()]; ok && !parked(b) {
			return false
		}
	}
	for _, nd := range s.nodes {
		if nd.closed {
			continue
		}
		if b, ok := blocks[nd.hdlGID.Load()]; ok && !strings.HasPrefix(state(b), "select") {
			return false
		}
		b, ok := blocks[nd.execGID.Load()]
		if !ok || parked(b) {
			continue
		}
		nd.c.QLock.Lock()
		empty := nd.c.Queue.Len() == 0
		nd.c.QLock.Unlock()
		if !empty {
			return false
		}
		for _, line := range strings.Split(b, "\n") {
			if k := strings.Index(line, "consensus.(*Consensus)."); k >= 0 && !strings.HasPrefix(line, "\t") {
				fn := line[k+len("consensus.(*Consensus)."):]
				if p := strings.Index(fn, "("); p >= 0 {
					fn = fn[:p]
				}
				if fn != "Executor" && fn != "getRequest" && fn != "QPop" {
					return false
				}
			}
		}
	}
	return true
}

// svrSeqs sums the slot numbers the live executors are on. It is read without synchronization, as a hint only:
// settled() treats a busy-waiting executor as idle, and a change here between two samples means it was not.
func (s *sim) svrSeqs() int {
	total := 0
	for _, nd := range s.nodes {
		if !nd.closed {
			total += nd.c.SvrSeq
		}
	}
	return total
}

func (s *sim) channelsEmpty() bool {
	for _, nd := range s.nodes {
		// toExec is left out: spare ProposalReply messages stay there until the executor's next proposal request
		if len(nd.fromExec) > 0 || len(nd.fromHdl) > 0 {
			return false
		}
		if !s.crashed[nd.id] && !nd.dead.Load() && len(nd.toHandler) > 0 {
			return false
		}
	}
	return true
}

// flush takes what the replicas sent during this tick, in a fixed order, and puts it on the network.
func (s *sim) flush(tick int) {
	s.mu.Lock()
	items := s.outbox
	s.outbox = nil
	s.mu.Unlock()
	sort.Slice(items, func(a, b int) bool {
		x, y := items[a], items[b]
		if x.sentAt != y.sentAt {
			return x.sentAt < y.sentAt
		}
		if x.from != y.from {
			return x.from < y.from
		}
		if x.handler != y.handler {
			return !x.handler
		}
		return x.seq < y.seq
	})
	for _, it := range items {
		if s.crashed[it.from] {
			s.dropped++
			s.record(traceEvent{Tick: tick, Ev: "drop", From: it.from, To: -1, Type: it.msg.Type.String(),
				Note: "sender crashed"})
			continue
		}
		if it.handler {
			to := int(it.msg.Phase)
			if to < 0 || to >= s.cfg.N {
				panic(fmt.Sprintf("faultsim: ProposalReply to replica %d", to))
			}
			s.send(it.from, to, true, it.msg, it.sentAt)
			continue
		}
		if it.msg.Type != message.ClientRequest && it.msg.Obj != nil {
			s.maxSlot = max(s.maxSlot, int(it.msg.Obj.SvrSeq))
		}
		if it.msg.Type == message.ProposalRequest {
			s.askedFor[it.from], s.answered[it.from] = int(it.msg.Value), false
		}
		if it.msg.Type == message.Proposal && it.msg.Obj != nil {
			slot := it.msg.Obj.SvrSeq
			if s.proposals[slot] == nil {
				s.proposals[slot] = map[int]message.ConsensusObj{}
			}
			s.proposals[slot][it.from] = *copyObj(it.msg.Obj)
		}
		for j := 0; j < s.cfg.N; j++ {
			s.send(it.from, j, false, message.Msg{Type: it.msg.Type, Phase: it.msg.Phase, Value: it.msg.Value,
				Obj: copyObj(it.msg.Obj)}, it.sentAt)
		}
	}
}

/*
crash stops replica r at tick: it receives nothing more, its later sends are dropped, and on each of its links a
random suffix of the messages still in flight is lost (so a broadcast can reach only some replicas).
*/
func (s *sim) crash(r, tick int, why string) {
	if s.crashed[r] {
		return
	}
	s.crashed[r] = true
	s.record(traceEvent{Tick: tick, Ev: "crash", From: r, To: -1, Note: why})
	s.halt(s.nodes[r])
	byLink := make([][]*envelope, s.cfg.N)
	kept := s.pending[:0]
	for _, e := range s.pending {
		switch {
		case e.to == r:
			s.drop(e, tick, "receiver crashed")
		case e.from == r:
			byLink[e.to] = append(byLink[e.to], e)
		default:
			kept = append(kept, e)
		}
	}
	for j, link := range byLink {
		sort.Slice(link, func(a, b int) bool { return link[a].linkSeq < link[b].linkSeq })
		cut := len(link) - int(s.rnd(5, r, j, 0)%uint64(len(link)+1))
		if s.cfg.LoseAll {
			cut = 0
		}
		kept = append(kept, link[:cut]...)
		for _, e := range link[cut:] {
			s.drop(e, tick, fmt.Sprintf("lost in crash of %d (link to %d)", r, j))
		}
	}
	s.pending = kept
}

// halt closes a replica's Done channel, so its routines return as they would if the process died.
func (s *sim) halt(nd *node) {
	if nd.closed {
		return
	}
	nd.closed = true
	close(nd.done)
	// an executor blocked in requestProposalAndWait does not watch Done
	select {
	case nd.toExec <- message.Msg{Type: message.ProposalReply, Value: math.MaxUint32,
		Obj: &message.ConsensusObj{ProId: sentinelProId, ProSeq: math.MaxUint32}}:
	default:
	}
}

func (s *sim) drop(e *envelope, tick int, why string) {
	s.dropped++
	ev := eventOf(tick, "drop", e)
	ev.Note = why
	s.record(ev)
}

func eventOf(tick int, kind string, e *envelope) traceEvent {
	ev := traceEvent{Tick: tick, Ev: kind, From: e.from, To: e.to, Type: e.msg.Type.String(), Phase: e.msg.Phase,
		Value: e.msg.Value}
	if o := e.msg.Obj; o != nil {
		ev.Seq, ev.ProId, ev.ProSeq, ev.IsNull = o.SvrSeq, o.ProId, o.ProSeq, o.IsNull
	}
	return ev
}

func (s *sim) record(ev traceEvent) {
	if s.cfg.TraceDir != "" {
		s.trace = append(s.trace, ev)
	}
}

func (s *sim) hashEvent(ev traceEvent) {
	h := fnv.New64a()
	fmt.Fprintf(h, "%x|%d|%d|%d|%s|%d|%d|%d|%d|%d|%t", s.hash, ev.Tick, ev.From, ev.To, ev.Type, ev.Phase, ev.Value,
		ev.Seq, ev.ProId, ev.ProSeq, ev.IsNull)
	s.hash = h.Sum64()
}

func copyObj(o *message.ConsensusObj) *message.ConsensusObj {
	if o == nil {
		return nil
	}
	c := *o
	c.CliIds = append([]uint32(nil), o.CliIds...)
	c.CliSeqs = append([]uint32(nil), o.CliSeqs...)
	c.Commands = append([]string(nil), o.Commands...)
	return &c
}

/*
shutdown closes Done and waits for every consensus routine to return. An executor blocked in
requestProposalAndWait (which does not watch Done) gets a sentinel ProposalReply, and once the executors are gone,
the ledger queues are drained so that a MsgHandler blocked on a full queue can reach its Done check. It returns
false if the routines did not all return within ten seconds.
*/
func (s *sim) shutdown() bool {
	s.stopping.Store(true)
	for _, nd := range s.nodes {
		s.halt(nd)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		all := true
		for _, nd := range s.nodes {
			all = all && nd.execDone.Load()
		}
		if all || time.Now().After(deadline) {
			break
		}
		time.Sleep(100 * time.Microsecond)
	}
	exited := make(chan struct{})
	go func() { s.wg.Wait(); close(exited) }()
	ok := true
Drain:
	for {
		select {
		case <-exited:
			break Drain
		case <-time.After(time.Millisecond):
			if time.Now().After(deadline) {
				ok = false
				break Drain
			}
			for _, nd := range s.nodes {
				if !nd.execDone.Load() {
					continue
				}
				for _, sl := range nd.led {
					select {
					case <-sl.Queue:
					default:
					}
				}
			}
		}
	}
	close(s.stop)
	s.fwdWg.Wait()
	if ok {
		for _, nd := range s.nodes {
			_ = nd.c.LogFile.Close()
			_ = nd.c.RoundDistLogFile.Close()
		}
	}
	return ok
}

// check reads the replicas' ledgers (after shutdown, so without races) and fills in a Result.
func (s *sim) check() Result {
	cfg := s.cfg
	res := Result{Profile: cfg.Name, N: cfg.N, F: cfg.F, Seed: cfg.Seed, Mutant: cfg.Mutant, Delivered: s.delivered, Dropped: s.dropped,
		Submitted: s.submitted, Required: len(s.required)}
	for r, c := range s.crashed {
		if c {
			res.Crashed = append(res.Crashed, r)
		}
	}
	s.panicMu.Lock()
	res.Panics = append(res.Panics, s.panics...)
	s.panicMu.Unlock()

	obs := observation{n: cfg.N, proposals: s.proposals, requests: s.requests}
	wrapped := false
	nullSlots := map[uint32]bool{}
	for _, nd := range s.nodes {
		ds := map[uint32]message.ConsensusObj{}
		if nd.c.SvrSeq >= int(cfg.LenLedger)-1 {
			wrapped = true
		}
		for k := 0; k <= nd.c.SvrSeq && k < len(nd.led); k++ {
			sl := nd.led[k]
			res.MaxPhase = max(res.MaxPhase, int(sl.Phase))
			if len(sl.RecvProposals) > 0 && sl.RecvProposalsMajT() >= config.Conf.MajorityPlusF {
				res.FastPathSlots++
			}
			if !sl.IsDone || sl.Decision.ProId == sentinelProId {
				continue
			}
			ds[uint32(k)] = sl.Decision
			if sl.Decision.IsNull {
				nullSlots[uint32(k)] = true
			}
		}
		obs.decisions = append(obs.decisions, ds)
	}
	res.NullSlots = len(nullSlots)
	res.Safety = checkSafety(obs)
	res.Duplicates = checkDuplicates(obs)

	for _, nd := range s.nodes {
		if s.crashed[nd.id] || nd.dead.Load() {
			continue
		}
		decided := map[string]bool{}
		for _, d := range obs.decisions[nd.id] {
			if !d.IsNull {
				decided[d.GetIdSeq()] = true
			}
		}
		res.Slots = max(res.Slots, len(obs.decisions[nd.id]))
		var missing []string
		for id := range s.required {
			if !decided[id] {
				missing = append(missing, id)
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			res.Stalls = append(res.Stalls, s.describeStall(nd, missing))
		}
	}
	res.Liveness = s.livenessTags(obs)

	switch {
	case wrapped:
		res.Outcome = "wrapped"
	case len(res.Safety) > 0:
		res.Outcome = "safety"
	case len(res.Panics) > 0:
		res.Outcome = "panic"
	case len(res.Stalls) > 0:
		res.Outcome = "stall"
	case len(res.Duplicates) > 0:
		res.Outcome = "duplicate"
	default:
		res.Outcome = "ok"
	}
	return res
}

// decided reports whether a slot has a real decision. The sentinel that unblocks an executor at shutdown also ends
// in epilogue, so a slot decided only by the sentinel was still undecided when the run stopped.
func decided(sl *ledger.Slot) bool {
	return sl.IsDone && sl.Decision.ProId != sentinelProId
}

// holds lists the requests a replica still holds: its pending queue plus the proposal of the slot it is working on.
func (s *sim) holds(nd *node) []string {
	var ids []string
	for _, o := range nd.c.Queue {
		ids = append(ids, o.GetIdSeq())
	}
	if seq := nd.c.SvrSeq; seq >= 0 && !decided(nd.led[seq]) {
		ids = append(ids, nd.led[seq].MyProposal.GetIdSeq())
	}
	return ids
}

// livenessTags names the patterns behind a run's stalls (see Result.Liveness).
func (s *sim) livenessTags(obs observation) []string {
	tags := map[string]bool{}
	live := []*node{}
	for _, nd := range s.nodes {
		if !s.crashed[nd.id] && !nd.dead.Load() {
			live = append(live, nd)
		}
	}
	for _, nd := range live {
		seq := nd.c.SvrSeq
		if seq >= 0 && !decided(nd.led[seq]) {
			for _, other := range live {
				if _, ok := obs.decisions[other.id][uint32(seq)]; ok {
					tags["decided-elsewhere"] = true
				}
			}
			if s.askedFor[nd.id] == seq && !s.answered[nd.id] {
				tags["proposal-reply"] = true
			}
		}
		for _, id := range s.holds(nd) {
			for _, other := range live {
				if !s.got[other.id][id] {
					tags["poison-request"] = true
				}
			}
		}
	}
	nulls := 0
	for _, ds := range obs.decisions {
		n := 0
		for _, d := range ds {
			if d.IsNull {
				n++
			}
		}
		nulls = max(nulls, n)
	}
	if nulls > 10*max(len(s.requests), 1) {
		tags["null-livelock"] = true
	}
	out := make([]string, 0, len(tags))
	for t := range tags {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

/*
describeStall says where a live replica stopped and, for each request it never decided, whether the request never
reached it, is still in its pending queue, or is marked for discarding.
*/
func (s *sim) describeStall(nd *node, missing []string) string {
	where := "idle"
	if seq := nd.c.SvrSeq; seq >= 0 && !decided(nd.led[seq]) {
		sl := nd.led[seq]
		where = fmt.Sprintf("slot %d undecided at phase %d round %d", seq, sl.Phase, sl.Round)
		if s.askedFor[nd.id] == seq && !s.answered[nd.id] {
			where += ", waiting for a ProposalReply"
		}
	}
	queued := map[string]bool{}
	for _, id := range s.holds(nd) {
		queued[id] = true
	}
	var parts []string
	for i, id := range missing {
		if i == 5 {
			parts = append(parts, fmt.Sprintf("and %d more", len(missing)-5))
			break
		}
		state := "queued"
		switch {
		case !s.got[nd.id][id]:
			state = "never received"
		case nd.c.Discard[id]:
			state = "marked discard"
		case !queued[id]:
			state = "not pending"
		}
		parts = append(parts, id+" "+state)
	}
	return fmt.Sprintf("replica %d (%s, queue %d) never decided: %s", nd.id, where, nd.c.Queue.Len(),
		strings.Join(parts, ", "))
}

// writeTrace saves the configuration, the result, the schedule, every network event and any panic stacks as JSON lines.
func (s *sim) writeTrace(res Result) string {
	if err := os.MkdirAll(s.cfg.TraceDir, 0o755); err != nil {
		return "error: " + err.Error()
	}
	name := fmt.Sprintf("n%d-%s-seed%d", s.cfg.N, res.Outcome, s.cfg.Seed)
	if s.cfg.Mutant != "" {
		name = s.cfg.Mutant + "-" + name
	}
	if res.Profile != "" {
		name = res.Profile + "-" + name
	}
	path := filepath.Join(s.cfg.TraceDir, name+".jsonl")
	f, err := os.Create(path)
	if err != nil {
		return "error: " + err.Error()
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	cfg := s.cfg
	cfg.TraceDir = ""
	_ = enc.Encode(map[string]interface{}{"config": cfg, "result": res})
	for _, p := range s.parts {
		_ = enc.Encode(map[string]interface{}{"partition": map[string]interface{}{"start": p.start, "end": p.end, "side": p.side}})
	}
	for _, ev := range s.trace {
		_ = enc.Encode(ev)
	}
	s.panicMu.Lock()
	for _, st := range s.stacks {
		_ = enc.Encode(map[string]string{"stack": st})
	}
	s.panicMu.Unlock()
	return path
}
