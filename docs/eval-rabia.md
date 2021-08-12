## Suggested setup

platform and machines: 

- Google Compute Engine VMs for Figure 4c: multi-zone throughput and latency
    - 3 server VMs one in us-east-1-b, one in us-east-1-c, and the last in us-east-1-d,
      e2-highmem-4 instances (Intel Xeon 2.8GHz, 4 vCPUs, 32 GB RAM)
    - 3 client VMs in us-east-1-b, each with 30 vCPUs and 120 GB RAM
    - OS: Ubuntu-1604
- CloudLab for all other evaluations
    - Instantiate the following [profile](https://www.cloudlab.us/show-profile.php?uuid=1af34047-fb02-11eb-84f8-e4434b2381fc).

Rabia's installation
- On Google Compute Engine: Use the script in [Section 2.1 of How to install and run Rabia](run-rabia.md#21-on-a-cluster-of-newly-instantiated-vms)
    
- CloudLab: Once the experiment profile noted above has been instantiated, you have two methods of completing the installation:
  1. SSH to your VMs using root@[hostname] and the id_rsa private key found in deployment/install. 
      1. Hostnames can be found in the "list view" portion of your experiment page.
  2. Use the web shell feature on Cloudlab to access each VM.
     
After choosing a method, it is same as GCP. Use the script in [Section 2.1 of How to install and run Rabia](run-rabia.md#21-on-a-cluster-of-newly-instantiated-vms) for each VM.

After installing Rabia on each VM, enter the `deployment/run` folder and 
call `. single.sh` to make sure the installation works; The script starts
a 3-server Rabia cluster on the single VM, and when it exits, one can see
a few liness of performance statistics are produced in `result.txt` in the
project's root folder.


## 6.1 Performance without Batching

### Rabia

- Follow the steps in [Section 4 of How to install and run Rabia](run-rabia.md#4-run-and-benchmark-rabia-on-a-cluster-of-vms)
to adjust `ServerIps`, `ClientIps`, and `Controller` entries in `profile0.sh` on EACH CloudLab server. 

- Run Rabia through profile `profile_sosp_table1.sh`. Specifically, edit the bottom of `multiple.sh` at the controller's
  VM (the first server VM):
      
      source ../profile/profile0.sh # required, load the variables and functions
      run_once  ${RCFolder}/deployment/profile/profile_sosp_table1.sh

- Again at the controller's VM, save `multiple.sh` and in the `deployment/run` folder, call `. single.sh`  to start the cluster.

- After 120-150 seconds, the run should be over. Check `result.txt` at the root folder of the controller VM to retrieve performance statistics.
  You may see a bunch of numbers separated by commas. You can copy them to a Google Sheet, select the first column, and 
  click "Data" -> "Split text to columns" -> "Separator: Comma" to get a clear read of the statistics.

### EPaxos, Paxos, EPaxos (NP), Paxos(NP)

TODO

## 6.2 Throughput vs. Latency

### Rabia

Following the steps in 6.1, run Rabia on CloudLab with profile `profile_sosp_4abc.sh`. 
Adjust the number of clients (parameter `NClients`) manually after a run to produce a series of data points.

Following the steps in 6.1, run Rabia on Google Cloud Platform with profile `profile_sosp_4d.sh`.
Adjust the number of clients (parameter `NClients`) manually after a run to produce a series of data points.

### EPaxos, Paxos

TODO


## 6.3 Varying Data Size

### Rabia

For all VMs, in `internal/config/config.go`, function `CalcConstants`, set 

    	c.KeyLen = 128
    	c.ValLen = 128

Following the steps in 6.1, run Rabia on CloudLab with profile `profile_sosp_4abc.sh`.
Adjust the number of clients (parameter `NClients`) manually after a run to produce a series of data points.
  

### EPaxos, Paxos

TODO

## 6.4 Integration with Redis

- Sync-Rep (1): requires two VMs, one as the Redis leader and the other as a follower; See steps [here](run-redis-sync-rep.md)

- Sync-Rep (2): requires three VMs, one as the Redis leader and the other two as followers;

- Rabia: Following the steps in 6.1, run Rabia on CloudLab with profile `profile_sosp_5a.sh` to generate the low bar and
  `profile_sosp_5b.sh` to generate the high bar.
  
- RedisRabia: requires three VMs, set `StorageMode` in `config.go` to 2 on all VMs, see steps [here](run-rabia.md#5-run-rabia-redis) for details.
  Following the steps in 6.1, run Rabia on CloudLab with profile `profile_sosp_5a.sh` to generate the low bar and
  `profile_sosp_5b.sh` to generate the high bar. Maybe reset `StorageMode` in `config.go` to 0.

- Redis-Raft: TODO