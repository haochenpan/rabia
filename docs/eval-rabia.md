## Performance without Batching

### Rabia

run Rabia through profile `profile_sosp_table1.sh`

### EPaxos, Paxos, EPaxos (NP), Paxos(NP)

todo

## Throughput vs. Latency

### Rabia

run Rabia though `profile_sosp_4abc.sh` and `profile_sosp_4d.sh`

### EPaxos, Paxos

todo


## Varying Data Size

### Rabia

For all VMs, in `internal/config/config.go`, function `CalcConstants`, set 

    	c.KeyLen = 128
    	c.ValLen = 128

run Rabia though `profile_sosp_4abc.sh` with NClients fixed to 200

### EPaxos, Paxos

todo

## Integration with Redis

[Run Sync-Rep](run-redis-sync-rep.md)

[Run Redis-Rabia](run-rabia.md) -- see section 5

Run Redis-Raft: todo