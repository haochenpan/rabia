# Rabia

## Introduction

We introduce Rabia, a simple and high performance framework for implementing state-machine replication (SMR) within a datacenter. The main innovation of Rabia is in using _randomization_ to simplify the design. Rabia provides the following two features: (i) It does not need any fail-over protocol and supports trivial auxiliary protocols like log compaction, snapshotting, and reconfiguration, components that are often considered the most challenging when developing SMR systems; and (ii) It provides high performance, up to 1.5x higher throughput than the closest competitor (i.e., EPaxos) in a favorable setup (same availability zone with three replicas) and is comparable with a larger number of replicas or when deployed in multiple availability zones.

Our SOSP paper, "[Rabia: Simplifying State-Machine Replication Through Randomization](https://dl.acm.org/doi/10.1145/3477132.3483582)," describes Rabia's design and evaluations in detail ([SOSP Artifact Review Summary](https://sysartifacts.github.io/sosp2021/summaries/rabia.html)) and earns three badges: artifact available, artifact evaluated, and artifact reproduced. 

#### Project Keywords: 
- state-machine replication (SMR), consensus, and formal verification

#### CCS Concepts: 
- Computer systems organization → Dependable and fault-tolerant systems and networks; 
- Computing methodologies → Distributed algorithms.

## Repository structure
- deployment, internal, roles, and `main.go`: Rabia's implementation in Go and the project's auxiliary code
- proofs: proof scripts for the core weak Multivalued consensus part of the Rabia protocol.
- redis-raft: redis-raft related code and instructions
- epaxos: compiled binaries of Paxos and EPaxos for cloudlab machines from various branches in [(E)Paxos](https://github.com/zhouaea/epaxos) and 
  [(E)Paxos-NP](https://github.com/zhouaea/epaxos-single) codebases + scripts to run them
- docs: documentations, see below

## Documentations

[]()
[How to install and run Rabia](docs/run-rabia.md) -- install and run Rabia on a single machine or a cluster of machines

[How to read Rabia's codebase](docs/read-rabia.md) -- an introduction to Rabia's implementation

[Package-level comments](docs/package-level-comments.md) -- contains all Go packages' comments, some design assumptions
and rationales, which can be served as an in-depth guide to this codebase. 

[Rabia's Roadmap and ToDos](docs/rabia-todo.md) -- for overarching objectives and and granular items

[Developer notes](docs/notes-for-developers.md) -- contains FAQs and some miscellaneous hints for developers

## Main contributors

Lewis Tseng, Joseph Tassarotti, Haochen Pan, Jesse Tuğlu, 
Neo Zhou, Tianshu Wang, Yicheng Shen, Andrew Chapman and Matthew Abbene -- Boston College

Roberto Palmieri -- Lehigh University

Zheng Xiong -- The University of Texas at Austin

