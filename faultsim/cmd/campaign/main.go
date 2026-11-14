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

// Command campaign runs many seeded faultsim runs and summarizes them.
//
//	campaign run -profiles delay,mixed -n 3,5 -seeds 1000 -out results.jsonl [-mutants fast-path] [-shard 0 -shards 4]
//	campaign summary results*.jsonl
//	campaign summary -compare replay.jsonl results*.jsonl
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"rabia/faultsim"
	"sort"
	"strconv"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: campaign run|summary [flags]")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "run":
		run(os.Args[2:])
	case "summary":
		summary(os.Args[2:])
	default:
		fmt.Fprintln(os.Stderr, "usage: campaign run|summary [flags]")
		os.Exit(2)
	}
}

func split(s string) []string {
	var out []string
	for _, x := range strings.Split(s, ",") {
		if x = strings.TrimSpace(x); x != "" {
			out = append(out, x)
		}
	}
	return out
}

func run(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	profiles := fs.String("profiles", strings.Join(faultsim.Profiles, ","), "fault mixes")
	ns := fs.String("n", "3,5", "cluster sizes")
	seeds := fs.Int("seeds", 100, "seeds per (mutant, profile, n)")
	start := fs.Int64("start", 1, "first seed")
	mutants := fs.String("mutants", "none", `comma-separated mutants; "none" is the unmodified implementation`)
	out := fs.String("out", "results.jsonl", "results file (appended)")
	traces := fs.String("traces", "", "folder for traces of runs that are not ok")
	shard := fs.Int("shard", 0, "this process runs the combinations whose index modulo shards equals shard")
	shards := fs.Int("shards", 1, "number of shards")
	_ = fs.Parse(args)

	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	f, err := os.OpenFile(*out, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	enc := json.NewEncoder(w)

	began := time.Now()
	idx, done := 0, 0
	for _, m := range split(*mutants) {
		if m == "none" {
			m = ""
		}
		for _, p := range split(*profiles) {
			for _, nStr := range split(*ns) {
				n, err := strconv.Atoi(nStr)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(2)
				}
				for k := 0; k < *seeds; k++ {
					idx++
					if (idx-1)%*shards != *shard {
						continue
					}
					cfg, err := faultsim.Profile(p, n, *start+int64(k))
					if err != nil {
						fmt.Fprintln(os.Stderr, err)
						os.Exit(2)
					}
					cfg.Mutant = m
					cfg.TraceDir = *traces
					r := faultsim.Run(cfg)
					if err := enc.Encode(r); err != nil {
						fmt.Fprintln(os.Stderr, err)
						os.Exit(1)
					}
					done++
					if done%50 == 0 {
						_ = w.Flush()
						fmt.Fprintf(os.Stderr, "shard %d: %d runs in %s\n", *shard, done, time.Since(began).Round(time.Second))
					}
				}
			}
		}
	}
	_ = w.Flush()
	fmt.Fprintf(os.Stderr, "shard %d: finished %d runs in %s\n", *shard, done, time.Since(began).Round(time.Second))
}

type key struct {
	mutant, profile string
	n               int
}

func (k key) String() string {
	m := k.mutant
	if m == "" {
		m = "none"
	}
	return fmt.Sprintf("| %s | %s | %d |", m, k.profile, k.n)
}

func load(paths []string) []faultsim.Result {
	var all []faultsim.Result
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1<<20), 1<<24)
		for sc.Scan() {
			var r faultsim.Result
			if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
				fmt.Fprintln(os.Stderr, p, err)
				os.Exit(1)
			}
			all = append(all, r)
		}
		_ = f.Close()
	}
	return all
}

func summary(args []string) {
	fs := flag.NewFlagSet("summary", flag.ExitOnError)
	compare := fs.String("compare", "", "a second results file of the same seeds, to measure replay agreement")
	_ = fs.Parse(args)
	all := load(fs.Args())

	outcomes := []string{"ok", "safety", "panic", "stall", "timeout", "wedged", "hang", "duplicate", "wrapped"}
	groups := map[key][]faultsim.Result{}
	var keys []key
	for _, r := range all {
		k := key{r.Mutant, r.Profile, r.N}
		if _, ok := groups[k]; !ok {
			keys = append(keys, k)
		}
		groups[k] = append(groups[k], r)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.mutant != b.mutant {
			return a.mutant < b.mutant
		}
		if a.profile != b.profile {
			return a.profile < b.profile
		}
		return a.n < b.n
	})

	fmt.Printf("%d runs\n\n", len(all))
	fmt.Print("| mutant | profile | n | runs |")
	for _, o := range outcomes {
		fmt.Printf(" %s |", o)
	}
	fmt.Println(" median wall ms | max phase | fast-path pairs |")
	fmt.Print("|---|---|---|---|")
	for range outcomes {
		fmt.Print("---|")
	}
	fmt.Println("---|---|---|")
	for _, k := range keys {
		rs := groups[k]
		count := map[string]int{}
		var walls []int64
		maxPhase, fast := 0, 0
		for _, r := range rs {
			count[r.Outcome]++
			walls = append(walls, r.WallMs)
			maxPhase = max(maxPhase, r.MaxPhase)
			fast += r.FastPathSlots
		}
		sort.Slice(walls, func(i, j int) bool { return walls[i] < walls[j] })
		fmt.Printf("%s %d |", k, len(rs))
		for _, o := range outcomes {
			fmt.Printf(" %d |", count[o])
		}
		fmt.Printf(" %d | %d | %d |\n", walls[len(walls)/2], maxPhase, fast)
	}

	fmt.Println("\nLiveness tags of runs that ended in stall or timeout:")
	fmt.Println("\n| mutant | profile | n | stalled runs | decided-elsewhere | poison-request | proposal-reply | null-livelock | untagged |")
	fmt.Println("|---|---|---|---|---|---|---|---|---|")
	for _, k := range keys {
		tags := map[string]int{}
		stalled := 0
		for _, r := range groups[k] {
			if r.Outcome != "stall" && r.Outcome != "timeout" {
				continue
			}
			stalled++
			if len(r.Liveness) == 0 {
				tags["untagged"]++
			}
			for _, t := range r.Liveness {
				tags[t]++
			}
		}
		if stalled == 0 {
			continue
		}
		fmt.Printf("%s %d | %d | %d | %d | %d | %d |\n", k, stalled, tags["decided-elsewhere"], tags["poison-request"],
			tags["proposal-reply"], tags["null-livelock"], tags["untagged"])
	}

	fmt.Println("\nFirst lines of safety violations and panics:")
	shown := map[string]int{}
	for _, r := range all {
		for _, line := range append(append([]string{}, r.Safety...), r.Panics...) {
			kind := strings.SplitN(line, ":", 2)[0]
			if r.Mutant != "" {
				kind = r.Mutant + " " + kind
			}
			if shown[kind] < 2 {
				shown[kind]++
				fmt.Printf("- %s/%s n=%d seed %d: %s\n", r.Mutant, r.Profile, r.N, r.Seed, line)
			}
		}
	}

	if *compare == "" {
		return
	}
	other := map[string]faultsim.Result{}
	for _, r := range load([]string{*compare}) {
		other[fmt.Sprintf("%s/%s/%d/%d", r.Mutant, r.Profile, r.N, r.Seed)] = r
	}
	matched, sameTrace, sameOutcome := 0, 0, 0
	for _, r := range all {
		o, ok := other[fmt.Sprintf("%s/%s/%d/%d", r.Mutant, r.Profile, r.N, r.Seed)]
		if !ok {
			continue
		}
		matched++
		if o.TraceHash == r.TraceHash {
			sameTrace++
		}
		if o.Outcome == r.Outcome {
			sameOutcome++
		}
	}
	fmt.Printf("\nReplay: %d runs matched, %d with the same delivery trace, %d with the same outcome\n",
		matched, sameTrace, sameOutcome)
}
