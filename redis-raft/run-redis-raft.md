# Redis Raft
## Installation
### Important Notes
1. This installation requires minimum 3 VMs (see NBs below)
2. These instructions assume you have the Rabia project cloned on the standard Cloudlab profile (deployment/env/cloudlab)
```shell
    sudo su && cd
    # clone the project to root
    git clone https://github.com/RedisLabs/redisraft.git

    # install the project's build deps
    cd ~/go/src/rabia/redis-raft && . install.sh

    # build the project to generate redisraft.so
    . build.sh
```
## Running
In the paper, we created a script to run RedisRaft across a cluster of nodes(multiple.sh), but we suggest the following manual method that makes the run methods clear.

### NBs:
1. This script assumes port 5001 is open and unused. You may use any free port you like.
2. <HOST (1..n)> represent the eth1 IPs found on each node after running ```ifconfig```. These typically start with ```10.10```
3. RedisRaft recommends an odd number of nodes in a cluster. We have tested on 3.
### Steps

In the first VM (please note the ```&``` operator, which makes it possible to run the first cmd in the background):
```shell
redis-server --bind <HOST 1> --port 5001 --dbfilename raft1.rdb --loadmodule ~/redisraft/redisraft.so raft-log-filename raftlog1.db addr <HOST 1>:5001 &
redis-cli -h <HOST 1> -p 5001 RAFT.CLUSTER INIT
redis-cli -h <HOST 1> -p 5001 RAFT.CONFIG SET raft-log-fsync no
```
In subsequent VMs (please note the ```&``` operator, which makes it possible to run the first cmd in the background):
```shell
redis-server --bind <HOST n> --port 5001 --dbfilename raft<n>.rdb --loadmodule ~/redisraft/redisraft.so raft-log-filename raftlog<n>.db addr <HOST n>:5001 &
redis-cli -h <HOST n> -p 5001 RAFT.CLUSTER JOIN <HOST 1>:5001
redis-cli -h <HOST n> -p 5001 RAFT.CONFIG SET raft-log-fsync no
```
To view the performance, in a new terminal window:
```shell
redis-benchmark -h <HOST 1> -p 5001 -t set,get -c 500 -n 1000 -d 16 -P 100 -q
```
To kill the cluster and wipe the rdb files associated with all the nodes:
1. Replace the IPs in ```SERVER_IPS``` on line 3 of ```manual_kill.sh``` with your eth1 IPs specified in NBs section above.
2. Adjust ```BASE_PORT``` if you used a port other than 5001.
3. In one of the VMs, run the following:
```shell
cd ~/go/src/rabia/redis-raft/multiple && . manual_kill.sh
```

### Known Issues
Seen in our testing and mentioned in the paper was the issue of constant leader election. See an example [here](https://github.com/haochenpan/rabia/blob/main/redis-raft/Redis-Raft%20Leader%20Election.png)
***In case the above link doesn't work, it refers to the "Redis-Raft Leader Election" png in the root of redis-raft subdir***