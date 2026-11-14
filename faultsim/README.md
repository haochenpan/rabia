# faultsim: seeded fault injection for Rabia's consensus layer

faultsim runs n copies of the unmodified consensus routines in
`roles/server/layers/consensus` (`Executor` and `MsgHandler`) in one process. It
connects them with an in-memory network that injects faults from a seeded
schedule. After each run it checks every decided slot against the safety
properties that `proofs/ivy/weak_mvc.ivy` and `proofs/coq/weak_mvc.v` prove for
weak MVC. Nothing in the consensus package is changed or mocked. The harness
only replaces the TCP layer, the proxy and the clients.

## What is checked

Per slot, over every replica's ledger after the run:

| check | property | source |
|---|---|---|
| agreement | no two replicas decide different values | Ivy `decision_full_val_agree`, Coq `agreement1` |
| null-split | no replica decides null while another decides a value | Coq `agreement2` |
| validity | a decided value was proposed for that slot by some replica | Ivy `decision_full_val_validity` |
| majority | a decided value was proposed for that slot by at least n/2+1 replicas | the Ivy invariant on `decision_full_val` and `member_maj` |
| integrity | a decided value has the commands its client sent | extra |

Proposals are counted only while their sender was alive. The majority threshold
is computed from the real n, so it stays right when a mutant changes the
constants in `config.Conf`.

The harness also reports liveness, separately from safety. A run is a **stall**
when a live replica has not decided every request sent through a proxy that
never crashed. It is a **timeout** when the replicas keep proposing past slot
10 × requests + 50. Stalls and timeouts are tagged with the pattern behind them:

- `decided-elsewhere`: a live replica waits in a slot that another live replica
  has decided.
- `poison-request`: a live replica's queue has a request that another live
  replica never received.
- `proposal-reply`: a live replica waits for a ProposalReply.
- `null-livelock`: more null slots than ten times the requests.

A request decided twice by one replica is reported as a **duplicate**. A panic
in a consensus routine (for example a "should not happen" check) is caught and
reported, and that replica is then treated as crashed.

## Fault model

This is the model the implementation assumes. It is not a stronger one.

- **Links are FIFO and lose nothing between live replicas**, as TCP is. Every
  message gets a delay, and it is never delivered before an earlier message on
  the same link. The executor depends on this: `wait()` panics with "should not
  happen 1" when a later-phase notification arrives first.
- **Delays** are uniform in a range. The `tail` and `mixed` profiles add a
  heavy-tailed extra delay to a fraction of messages.
- **Partitions** split the replicas into two groups for a stretch of ticks.
  Messages across the split wait until it heals. They are not dropped, because
  Rabia has no reconnect logic.
- **Crash-stop** of up to f replicas. A crashed replica's routines return, it
  receives nothing more, and its later sends are dropped. On each of its links, a
  random suffix of the messages still in flight is lost, so its last broadcast
  can reach only some replicas. There is no recovery, because the
  implementation has none.
- **Clients** send requests round-robin through the replicas' proxies. Each
  request is broadcast over the proxy's links like any other message.

Virtual time is counted in ticks. At each tick the router does three things.
It delivers at most one due message to each live replica. It waits until every
replica has reacted. Then it gives each message they sent a delivery tick. A
send's tick is the tick of the delivery that caused it.

"Reacted" is read from the goroutines themselves, using one `runtime.Stack`
snapshot per sample. Each MsgHandler must be parked in its `select` with an
empty input channel. Each Executor must be parked in `wait` or
`requestProposalAndWait`, or busy-waiting in `getRequest` with an empty pending
queue. Every forwarder must be parked. These must hold, with no new sends, on
two samples in a row. A real-time quiet window was tried first and was not
enough: a replica's reaction sometimes took tens of milliseconds, mostly while
its MsgHandler waited for the pending-queue lock that the busy-waiting Executor
keeps taking.

## Profiles

| profile | delays (ticks) | heavy tail | partitions | crashes |
|---|---|---|---|---|
| delay | 1-10 | none | none | none |
| tail | 1-5 | 10% of messages get +1..200 | none | none |
| partition | 1-10 | none | 2, up to 150 ticks | none |
| crash | 1-10 | none | none | f |
| mixed | 1-10 | 5% get +1..200 | 2, up to 150 ticks | f |

Each run sends 10 requests, about one every 4 ticks. f is (n-1)/2, as in the
SOSP profiles under `deployment/profile`.

## Negative controls

A checker that never fires proves nothing, so `Mutants` breaks one quorum
constant in `config.Conf` for every replica:

- `fast-path`: decides in phase 0 on n/2+1 matching proposals in place of n/2+f+1.
- `low-majority`: a value counts as a majority with n/2 proposals.
- `weak-decide`: decides on one matching vote in place of f+1.

The campaign reports how often each mutant is caught. `TestMutantsAreCaught`
fails if one goes unnoticed.

## What a seed fixes

A seed fixes the request schedule, the crash and partition schedule, every
message's delay and the suffix a crash loses. Delays come from a hash of the
seed and the message's place on its link, so they don't depend on which tick a
send is noticed in. Because the router waits for every replica to settle before
it moves on, a replica sees the same inputs in the same order on every replay,
and the run replays to the same delivery trace. The campaign checks this by
running seeds twice (`campaign summary -compare`). `TestMutantsAreCaught` and
`TestReplayIsStable` depend on it. Every run that is not ok writes its full
delivery trace when `-traces` is set.

## Scripted schedules

Three tests pin liveness problems the campaign found to a fixed three-replica
schedule with one crash (f = 1). Each test has a control run without the crash,
which must finish.

- `TestDecisionNotRelayed`: replica 2 decides and crashes. Its Vote and
  Decision to replica 0 are lost. Replica 1 is behind, gets the Decision before
  it votes, decides without voting and does not pass the Decision on. Replica 0
  waits forever for a second phase-0 vote. Replica 1 then waits forever in the
  next slot for replica 0's proposal.
- `TestPoisonRequest`: replica 2's proxy sends request 2-0 to replicas 0 and 2
  and crashes before the copy to replica 1 arrives. 2-0 heads replica 0's
  pending queue from then on. Replica 1 never has it and proposes another
  request. Neither value gets a majority, so every slot is null, both proposals
  go back into the queues, and the next slot repeats this.

- `TestNoLiveWitness`: replicas 0 and 1 propose 0-0, but each counts only one
  copy of it before it stops at n-f proposals. Replica 2 counts two copies and
  votes 1, so every replica decides a value. Replicas 0 and 1 don't know which
  value, so they ask with a ProposalRequest. Only a replica that counted a
  majority answers, and replica 2 has crashed. They wait forever, although both
  of them proposed 0-0.

All three tests assert the current behaviour, so a fix will make them fail and
they will need flipping.

## Running it

```bash
go test ./faultsim/                 # checker, safety, mutants, the scripted stalls, replay
go test -short ./faultsim/          # fewer seeds
go run ./faultsim/cmd/campaign run -profiles mixed -n 3,5 -seeds 200 -out r.jsonl -traces traces/
go run ./faultsim/cmd/campaign summary r.jsonl
```

`campaign run -shard i -shards k` splits a campaign over k processes. Runs in
one process share `config.Conf`, so they run one at a time.

## Known limitations

- Runs share the global `config.Conf`, so one process runs one cluster at a time.
- Idle executors busy-wait in `getRequest`, as they do in a deployment. That
  costs CPU in the harness, and the stop-the-world stack snapshot costs about
  0.2 ms per sample. A run takes 0.2 to 4 seconds.
- Settling assumes a busy-waiting Executor with an empty queue has nothing to
  do. One that has just popped the last request and not yet sent its proposal
  looks the same for a few instructions. Two samples in a row, plus a check that
  no executor changed slot between them, make a wrong call unlikely, but not
  impossible.
- The harness reads ledgers only after the routines have exited. It is not
  `-race` clean, because the consensus package itself is not. See "Race
  Conditions Documented" in `consensus.go`. `RecvProposalsMajV` and
  `RecvProposalsMajT` also sort `RecvProposals` in place from both routines.
