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
	"fmt"
	"rabia/internal/message"
	"reflect"
	"sort"
)

/*
observation is what the checker sees after a run: every replica's decisions, the proposals that replicas sent
while they were alive, and the requests clients submitted. It is kept apart from the simulator so the checker can
be tested on hand-made observations.
*/
type observation struct {
	n         int
	decisions []map[uint32]message.ConsensusObj       // per replica: slot -> decision
	proposals map[uint32]map[int]message.ConsensusObj // slot -> replica -> proposal it sent for that slot
	requests  map[string]message.ConsensusObj         // request id -> request as the client submitted it
}

func objString(o message.ConsensusObj) string {
	if o.IsNull {
		return "null"
	}
	return o.GetIdSeq()
}

/*
checkSafety returns one line per violated safety property. The properties are the ones proved for weak MVC in
proofs/ivy/weak_mvc.ivy and proofs/coq/weak_mvc.v, stated per slot:

agreement: no two replicas decide different non-null values (Ivy decision_full_val_agree, Coq agreement1)
null-split: no replica decides null while another decides a value (Coq agreement2)
validity: a decided value was proposed for that slot by some replica (Ivy decision_full_val_validity)
majority: a decided value was proposed for that slot by a majority of the replicas (the Ivy invariant on

	decision_full_val and member_maj)

integrity is an extra check: a decided value has the commands of the request a client submitted under that id.
The majority threshold is always n/2+1 for the real n, so it stays correct when a mutant changes the constants in
config.Conf.
*/
func checkSafety(obs observation) []string {
	var out []string
	majority := obs.n/2 + 1

	slots := map[uint32]bool{}
	for _, ds := range obs.decisions {
		for s := range ds {
			slots[s] = true
		}
	}
	order := make([]uint32, 0, len(slots))
	for s := range slots {
		order = append(order, s)
	}
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })

	for _, s := range order {
		var first *message.ConsensusObj
		firstNode, nullNode, valNode := -1, -1, -1
		seen := map[string]bool{}
		for r, ds := range obs.decisions {
			d, ok := ds[s]
			if !ok {
				continue
			}
			if d.IsNull {
				if nullNode < 0 {
					nullNode = r
				}
				continue
			}
			if valNode < 0 {
				valNode = r
			}
			if first == nil {
				v := d
				first, firstNode = &v, r
			} else if !message.ProxySeqIdEqual(first, &d) {
				out = append(out, fmt.Sprintf("agreement: slot %d: replica %d decided %s, replica %d decided %s",
					s, firstNode, objString(*first), r, objString(d)))
			}
			id := d.GetIdSeq()
			if seen[id] {
				continue
			}
			seen[id] = true
			if req, ok := obs.requests[id]; !ok {
				out = append(out, fmt.Sprintf("validity: slot %d: replica %d decided %s, which no client submitted", s, r, id))
			} else if !reflect.DeepEqual(req.Commands, d.Commands) || !reflect.DeepEqual(req.CliIds, d.CliIds) ||
				!reflect.DeepEqual(req.CliSeqs, d.CliSeqs) {
				out = append(out, fmt.Sprintf("integrity: slot %d: replica %d decided %s with commands %v, the client sent %v",
					s, r, id, d.Commands, req.Commands))
			}
			support := 0
			for _, p := range obs.proposals[s] {
				if message.ProxySeqIdEqual(&p, &d) {
					support++
				}
			}
			if support == 0 {
				out = append(out, fmt.Sprintf("validity: slot %d: replica %d decided %s, which no replica proposed for this slot",
					s, r, id))
			} else if support < majority {
				out = append(out, fmt.Sprintf("majority: slot %d: replica %d decided %s, proposed by %d replicas, a majority is %d",
					s, r, id, support, majority))
			}
		}
		if nullNode >= 0 && valNode >= 0 {
			out = append(out, fmt.Sprintf("null-split: slot %d: replica %d decided null, replica %d decided %s",
				s, nullNode, valNode, objString(obs.decisions[valNode][s])))
		}
	}
	return out
}

/*
checkDuplicates returns one line per request that a replica decided in more than one slot. This is not one of the
proved properties (weak MVC is per slot), but a replicated log that runs a command twice is worth reporting.
*/
func checkDuplicates(obs observation) []string {
	var out []string
	for r, ds := range obs.decisions {
		where := map[string][]uint32{}
		for s, d := range ds {
			if !d.IsNull {
				where[d.GetIdSeq()] = append(where[d.GetIdSeq()], s)
			}
		}
		ids := make([]string, 0, len(where))
		for id, ss := range where {
			if len(ss) > 1 {
				ids = append(ids, id)
			}
		}
		sort.Strings(ids)
		for _, id := range ids {
			ss := where[id]
			sort.Slice(ss, func(i, j int) bool { return ss[i] < ss[j] })
			out = append(out, fmt.Sprintf("duplicate: replica %d decided %s in slots %v", r, id, ss))
		}
	}
	return out
}
