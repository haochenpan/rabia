# Rabia

## Introduction

Rabia is a simple and high performance framework for implementing state-machine replication (SMR) within a datacenter. 
The main innovation of Rabia is in using randomization to simplify the design. Rabia provides the following two features: 
(i) It does not need any fail-over protocol and supports trivial auxiliary protocols like log compaction, snapshotting, 
and reconfiguration, these components often considered the most challenging when developing SMR systems; and (ii) It
provides high performance, up to 1.5x higher throughput than the closest competitor (i.e., EPaxos) in an ideal setup 
(same availability zone with 3 replicas) and comparable with a larger 𝑛 or when deployed in multiple zones. 

Our SOSP paper, "Rabia: Simplifying State-Machine Replication
Through Randomization," describes Rabia's design and evaluations in detail.

#### Project Keywords: state-machine replication (SMR), consensus, formal verification


## Documentations

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

