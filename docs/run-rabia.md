# Install and Run Rabia

## Table of contents
- [Suggested evaluation setups](#1-suggested-evaluation-setups)
- [Install Rabia](#2-install-rabia)
- [Run Rabia on a single VM](#3-run-rabia-on-a-single-vm)
- [Run and benchmark Rabia on a cluster of VMs](#4-run-and-benchmark-rabia-on-a-cluster-of-vms)
- [Run Rabia-Redis](#5-run-rabia-redis)
- [If something goes wrong](#6-if-something-goes-wrong)

## 1. Suggested evaluation setups

### 1.1 On Google Compute Engine

- Hardware: General-purpose E2 VMs
    - each server VM should have >=4 CPUs and 8 GB or more memory to support a Rabia server (might be a overkill)
    - each client VM should have 30 or more CPUs an 60 GB or more memory to support 100+ Rabia clients
- Operating System: Ubuntu 18.04 LTS (preferred) or Ubuntu 16.04 LTS

### 1.2 On CloudLab

- Hardware: Use the [profile](https://www.cloudlab.us/show-profile.php?uuid=1af34047-fb02-11eb-84f8-e4434b2381fc)
- Operating System: Ubuntu 18.04 LTS (preferred) or Ubuntu 16.04 LTS (Default OS in the above profile)

### 1.3 About the root access, ARM machine, and the default installation path

The root access is only required in installing Rabia on fresh VM; if the VM has Go and Python installed (see the comments
in `deployment/install/install.sh` for version requirements), one only needs to install Rabia and the required Go packages,
which does not require the root access. Rabia runs does not require the root access.

Do not use ARM machines with the default installation script; If you are on a linux that is not AMD64,
you need to download another version of Golang that is different from the one defined in
`deployment/install/install.sh`. You can use `uname -a` to check kernel release and hardware information. Then,
pick a version that is suitable for your machine among [all Golang versions](https://golang.org/dl/).
After that, you need to modify the variable named `go_tar` in `install.sh` to run the script properly.

`install.sh` and other Shell files of Rabia assume this project should be installed under `~/go/src`. Replace the 
occurrences of this path in the snippet below and in all Shell scripts if you wish to install this project under another 
directory. In a near future update, we will provide users a more convenient way to define the installation path.  


## 2. Install Rabia

### 2.1 on a cluster of newly instantiated VMs

On each VM you want to install Rabia, do the following:

```shell script
# 1. download the project to the default path (see section 1.3)
mkdir -p ~/go/src && cd ~/go/src
git clone https://github.com/haochenpan/rabia.git

# 2. install Rabia and its dependencies
cd ./rabia/deployment
. ./install/install.sh

# 3. check the installation:   
cd ./run
. single.sh          # the default parameter runs a Rabia cluster on a single machine for 20 seconds
cat ../../result.txt # inspect the performance statistics; one should see a few lines of file output
. clear.sh           # remove logs, result.txt, and the built binary
```

Now, each VM can hold one or more Rabia servers or clients. In a benchmarking setup, a Rabia controller is also required.
See the steps to benchmark below and more explanations [here](read-rabia.md) and [here](package-level-comments.md)


### 2.2 on a cluster of VMs that have Go and Python installed

Before running `install.sh`, remove `install_python` and `install_go` function calls in `install.sh`. Run `install.sh` 
and double check `PATH`, `GOPATH`, and `GO111MODULE` variables are set properly as the `install_go` function does.

Rabia's installation script installs Go 1.15.8 and the latest stable Python3.8, so if some unknown problems occurs
during installing/running Rabia, maybe try these versions of dependencies. 

`install.sh` also installs a recent version of Redis by default. To disable this feature, remove the `install_redis` 
function call at the bottom of `install.sh`.


## 3. Run Rabia on a single VM

> Before benchmarking Rabia on a cluster of VMs, running a Rabia cluster on a single VM is highly recommended as this
> practice helps you to get familiar to Rabia's scripts, parameters, and profile settings.


Steps to run:

- Read the header comments of `deployment/profile/profile0.sh` to select desirable parameters to run. 
  The default parameters should work.
- On terminal, enter the `deployment/run` folder, start the stand-alone cluster (i.e., a Rabia cluster on a single VM) 
  by entering `. single.sh`. After a few seconds or a few minutes, the terminal program should exit, and logs are 
  generated in the `logs` folder.
- Check `result.txt` after a run to retrieve performance statistics (`cat ../../result.txt`). You can copy them to a Google Sheet, 
  select the first column, and click "Data" -> "Split text to columns" -> "Separator: Comma" to get a clear read
  of statistics
- Type `. clear.sh` to remove the log folder, result.txt, and the built binary.

When you want to try out different parameters, read header comments of `profile0.sh`. And it is always good to start with small
parameters (e.g., a few clients, small client batch sizes). When these runs are successful, you can then try some large
parameters (e.g., many clients and large client batch size). 

A Rabia cluster must consist of 3 or more Rabia servers,
and 1 or more Rabia clients, so don't set NServers to 2 or NClients to 0.

Most Rabia parameters are made adjustable in `profile0.sh`, and `internal/config/config.go` contains some constants that
are less often adjusted.

Note: for now, scripts in the `deployment/run` folder should be invoked when the current directory is this folder; for 
example, do `. single.sh` and `. clear.sh`, but don't do `. ./run/single.sh`, `. ./run/clear.sh`.

## 4. Run and benchmark Rabia on a cluster of VMs

Say you want to run 3 Rabia servers on 3 server VMs and 120 Rabia clients on 3 client VMs.

- Install Rabia on each of the six VMs (see Section 2) or install Rabia on one VM, test a Rabia cluster indeed works on 
  it, and use the image/snapshot of this VM to spawn other VMs.
- Update the cluster configurations and profile selection on the six VMs
    - Download this repository to your developer machine (e.g., your Mac). 
    - Open `profile0.sh`, modify `ServerIps`, `ClientIps`, and `Controller` entries:
    - Fill `ServerIps` with 3 server VMs' internal IPs. e.g., `ServerIps=(10.142.0.105 10.142.0.106 10.142.0.107)`
    - Fill `ClientIps` with 3 client VMs' internal IPs. e.g., `ClientIps=(10.142.0.108 10.142.0.109 10.142.0.110)`
    - Let `Controller` be the **first server VM**'s IP:some unused port, e.g., `Controller=10.142.0.105:8070` (the first
      server VM is the VM with the first ip in `ServerIps`, so on and so forth).
    - `multiple.sh` supports three different executions. By default, the `run_once` function is executed. 
    Edit the bottom of `multiple.sh` to choose your execution mode:
      
    - `run_once` will execute Rabia using the configuration in profile0.sh
    - `visual_benchmark`will run multiple configurations in one execution of the shell script, these configurations are 
      intended to test if the cluster is running properly
    - `cluster_run` will also run multiple configurations in one execution of the shell script, but these configurations
      intended to conduct multiple benchmarking runs
    - Later, feel free to create profiles in the `deployment/profile` folder and use them in either `visual_benchmark` 
      or `cluster_run` through `run_once ${RCFolder}/deployment/profile/profileX.sh` function calls. In general, you 
      only need to enter a few variables (selected from Section 2 of `profile0.sh`) in`profileX.sh` as this profile 
      is intended to run the cluster with the base parameters in `profile0.sh` and overridden parameters in `profileX.sh`
    - Upload `multiple.sh` (if anything changed), `profile0.sh`, and any other edited profiles to all VMs - check 
      the section "How to upload files from your developer machine to a cluster of VMs" in 
      [Developer Notes](notes-for-developers.md).
    > Again, if you change multiple.sh or profileX.sh, please upload the new files to all VMs

- SSH to the **first server VM**
    - method 1: through your VM hypervisor's interface
    - method 2: through the private key from your developer machine (e.g., your Mac)
        - on your developer machine, go to the `deployment/install` folder
        - run `chmod 400 id_rsa`
        - run `ssh -i id_rsa <first-server-VM-user-name>@<first-server-VM-public-IP>`, note that the user must be the 
          same as the user that has the rabia directory in `~/go/src`
        
- At the **first server VM**
    - Go to `~/go/src/rabia/deployment/run`
    - Run `. multiple.sh`
    - Check the `logs` and `result.txt` for performance statistics when the program exits
    - If you want to try out a different profile after this run, please update the changed file to all VMs.


Let's label server and client routines as server-0, server-1, server-2, client-0, client-1, ..., and client-119. The
default settings put server-0 at the first server VM, server-1 at the second server VM, and server-2 at the third server
VM. Also, we have client-0, -3, -6, ..., -117 at the first client VM, client-1, -4, -7, ..., -118 at the second client
VM, and client-2, -5, -8, ..., -119 at the third client VM; Each client VM has 40 clients. Read comments
at `profile0.sh`'s header for instructions on setting servers and clients with other parameters.

Rabia's `multiple.sh` assumes the project has been installed under the **same** directory on **all** VMs; The directory
is `<first-server-VM-user-name>/go/src/rabia`. That requires <first-server-VM-user-name> == <second-server-VM-user-name>
== ... <last-client-VM-user-name>. The username(s) can be checked through `echo $USER`. If more than one operator needs
to manipulate these VMs, it is recommended to install Rabia under the root directory -- type `sudo su` before installing
Rabia and henceforth <first-server-VM-user-name> becomes `root`. Otherwise, <first-server-VM-user-name> is usually
myName_myInstitution_[com | edu | org]

In benchmarking for accurate performance statistics, set `RCLogLevel` in `profile0.sh` to `warn` in order to
output fewer log entries.

When `RCLogLevel` in `profile0.sh` is `debug`, proxies logs all slot decisions sequentially. The Python analysis code in
the `deployment/analysis` folder checks whether proxy-level logs are consistent, if it doesn't Python code raises an
Exception immediately. See function `load_proxy_info` in `deployment/analysis/analysis.py`

The `run_once` function in `multiple.sh` and `single.sh` makes following function calls to invoke Python script to calculate some
performance statistics and write them in `result.txt`:

    python3.8 ${RCFolder}/deployment/analysis/analysis.py ${RCFolder}/logs 1>>${RCFolder}/result.txt

If you check `result.txt`, you may see a bunch of numbers separated by commas. Add `print-title` flag to see what they
represent of or add `print-round-dist` flag to see the distribution of different numbers of rounds (an internal 
statistic of the algorithm):

    python3.8 ${RCFolder}/deployment/analysis/analysis.py ${RCFolder}/logs print-title print-round-dist 1>>${RCFolder}/result.txt

## 5. Run Rabia-Redis

### 5.1 Start stand-alone Redis instances
for each Rabia server instance, start a stand-alone Redis instance on the VM (n server requires n Redis instance),
and maybe start a client to monitor the redis DB.

If your Redis is installed through `install.sh`, go to the Redis' root folder and enter:
```shell
# on server-0
src/redis-server --port 6379 --appendonly no --save "" --daemonize yes
src/redis-cli -p 6379
# on server-1
src/redis-server --port 6380 --appendonly no --save "" --daemonize yes
src/redis-cli -p 6380
# on server-2
src/redis-server --port 6381 --appendonly no --save "" --daemonize yes
src/redis-cli -p 6381
```

### 5.2 Setting `StorageMode` and `RedisAddr` in config.go

Port numbers like 6379, 6380, 6381 are hardcoded in function `loadRedisVars()` in `config.go`;
If these ports are not available on your VMs, please assign different ports to Rabia instances and modify `c.RedisAddr`
in `loadRedisVars()`. If the cluster has 5 servers instead of three, append two addresses to the variable `c.RedisAddr`:

    c.RedisAddr = []string{"localhost:6379", "localhost:6380", "localhost:6381", "localhost:6382", "localhost:6383"}

We are using localhost because we don't want Redis instances recognize each other; Rabia will act like the
communication layer of these stand-alone Redis instances.

In config.go, adjust the `c.StorageMode` in `loadRedisVars()`:

- 0: no Redis, use the default dictionary object as the storage
- 1: use Redis' GET and SET commands only -- a consensus obj produces proxybatchsize * clientbatchsize GET and SET
  commands
- 2: use Redis' MGET and MSET commands only -- a consensus obj produces at most two commands, one is MSET, the other is MGET

> Note: in the MGET-MSET mode, all client write requests in a consensus object will be batched to a Redis command, and
> all read requests will be batched to another command, these two requests are executed sequentially (write first then
> read) without no pipelining. Please be aware of a potentially undesired consequence: for example, if the client batch
> size is 2, and a client issues a read request on key1 and then a write request to key1. The write request is executed
> before the read.

Then, adjust NClients, ProxyBatchSize, ClientBatchSize (through profile Shell files), KeyLen, ValLen (through `config.go`)
... as you usually do. Upload `config.go` and `profileX.sh` to all VMs if anything is changed.

Again, make sure you have started (stand-alone) redis servers before running Rabia with `c.StorageMode` set to a non-zero value!

After a run, you may want to type `keys *` in each redis-cli's terminal to see whether keys are written to the DB. Note: this operation takes a
significantly long time if there are many keys. Finally, run `flushall` to truncate the DB to prepare for the next clean run.

When you intend to run Rabia without Redis, set `c.StorageMode` on a VMs. Admittedly, this setting is somewhat
inconvenient, and we aim to fix this in the next major update.

### 5.3 Run Redis Synchronous Replication

We have implemented Redis synchronous replication for the purpose of comparison. See [code](https://github.com/YichengShen/redis-sync-rep) and [instructions on running the code](./run-redis-sync-rep.md).

## 6. If something goes wrong

> Always go to check the first error on the terminal; Scroll your terminal up, maybe there’s an error above.

If the program is running, at the controller's terminal, try to interrupt the program by pressing control/command + C.

(If interrupt signal does not work / for sanity) If you started Rabia from `single.sh`, open another terminal, enter
the `deployment/run` folder, call `. kill.sh` to stop all routines. If you started Rabia from `multiple.sh`, open
another terminal, SSH to the first server VM, enter the `deployment/run` folder, call `. multiple_kill.sh` to stop all
routines.

(for sanity) run `. clear.sh` inside the `run` folder to clear logs, the built binary, and `result.txt`. The
cluster-equivalent version is the `multi_reset_folder` function call defined in `profile0.sh`. Type `multi_reset_folder`
after a successful/unsuccessful run in the terminal will invoke this function.

### 6.1 Some instances are up while others are not

Double check whether all instances have the same profile files and the `multiple.sh` file. Try `ps -fe | grep rabia` to
see whether there are lingered instances. Stopping and restarting all VMs seems to be the ultimate solution.


### 6.2 Maybe the Shell code goes wrong

Check whether you have correct server internal addresses in `profile<x>.sh` and on GoLand's deployment tab (if you use
GoLand).

- A telltale sign that this is the case is if the execution is stuck for a while and takes a long time to output the Rabia logo

Ensure that all VMs have installed the same profiles, since `profile<x>.sh` will be read at every machine before Goroutines are instantiated.

- You can check this on each VM with `head -120 ~/go/src/rabia/deployment/profile0.sh` or `cat ~/go/src/rabia/deployment/profile0.sh`

Check whether VMs can `ping` each other, and check the first server VM can SSH to other VMs through the project SSH key.
`install.sh` should have put the public key of `deployment/install/id_rsa` in `~/.ssh/authorized_keys` on each VM, so
the first server VM can `deployment/install/id_rsa` (the private key) to SSH to other server.
- You can check if each VM has indeed installed the public key with `cat ~/.ssh/authorized_keys`

Sometimes sshd is dead/stuck for no reason (very rare), restart the cluster may solve the problem

### 6.3 If the Go program does not terminate

Check the terminal, see if there are panics logs. Log moves very fast, make sure you read all logs for this run -- maybe
there’s a panic log, and a lot of normal logs below. Try `. kill.sh` or `. multiple_kill.sh` as suggested above.

Review your recent changes to the code base.

Some parameters may cause an error. Check whether your parameters in `profile0.sh` has followed the suggested range in
the comments. Always start some experiments with small number of clients, small batch sizes, etc -- so then you know
whether that is a software problem or scalability problem.

### 6.4 Other miscellaneous problems

- `panic: mkdir .../go/src/rc3/logs: not a directory`

    Under rare circumstances, `src/rc3/logs` will be wrongly initialized as a file instead of a folder. 
    Run `clear.sh` and or delete the `logs` file manually
  
