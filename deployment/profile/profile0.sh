: <<'END'
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

Shell configurations explained with design rationales

ServerIps:
    Rabia servers' IP addresses (usually internal IPs)
    A note on ServerIps:
        If the length of ServerIps is less than the number of servers (set below), say len(ServerIps) = 5 and NServers = 7:
            Rabia server 0 and server 5 are instantiated on the 1st VM (i.e., the VM with the first IP);
            Rabia server 1 and server 6 are instantiated on the 2nd VM;
            Rabia server 2 is instantiated on the 3rd VM;
            Rabia server 3 is instantiated on the 4th VM;
            Rabia server 4 is instantiated on the 5th VM.
        If len(ServerIps) = 7 and NServers = 5:
            Rabia server 0 is instantiated on the 1st VM;
            Rabia server 1 is instantiated on the 2nd VM;
            Rabia server 2 is instantiated on the 3rd VM;
            Rabia server 3 is instantiated on the 4th VM;
            Rabia server 4 is instantiated on the 5th VM;
            6th VM has no Rabia server instantiated;
            7th VM has no Rabia server instantiated.

ClientIps:
    Rabia clients' IP addresses (usually internal IPs)
    A note on ClientIps, if Rabia_ClientDistributingMethod is set to modulo
        If the length of ClientIps is less than the number of clients (set below), say len(ClientIps) = 3 and NClients = 20:
            Rabia clientids 0, 3, 6, 9, 12, 15, 18 are instantiated on the 1st client VM (i.e., the VM with the first IP);
            Rabia clientids 1, 4, 7, 10, 13, 16, 19 are instantiated on the 2nd client VM
            Rabia clientids 2, 5, 8, 11, 14, 17 are instantiated on the 3nd client VM
        If len(ServerIps) = 3 and NClients = 2:
            Rabia client 0 is instantiated on the 1st VM;
            Rabia client 1 is instantiated on the 2nd VM;
            3rd VM has no Rabia clientid instantiated;

Controller:
    the benchmark controller's address (IP:port)

ProxyStartPort:
    Rabia server 0's proxy listener port, 1's proxy listener port is this number + 1, and so on so forth

NetworkStartPort:
    Rabia server 0's peer networking port, 1's peer networking is this number + 1, and so on so forth
    Why we ask ports for server 0 only:
        If we want to run Rabia servers and clients on a single VM, each of server/client needs to have unique ports;
        If we want to run Rabia servers and clients on a set of VMs, each of server/client may or may not use unique ports.
        So the setting ports is complicated and error-prone. To simplify the process, we decided that a user only need to 
        specify the ports for the first server and let Shell code figure out some always non-conflicting ports for other servers.

User:
    VM's user, e.g., root, name_bc_edu...

RCFolder:
    the path to the project's root directory
    NOTE: ending the path with a slash (/) is not required

RCLogLevel:
    warn | debug | info, only messages of and above this level will be logged
    
Rabia_ClosedLoop: 
    true | false, whether initiate closed-loop clients or open-loop clients

Rabia_ClientDistributingMethod:
  modulo: 
    clients then are distributed evenly across servers. 
    If there are 5 servers and 3 clients: 
        server 0 gets client 0
        server 1 gets client 2
        server 2 gets client 2;
    If there are 3 servers and 5 clients: 
        server 0 gets client 0, 3 
        server 1 gets client 2, 4
        server 2 gets client 2.
  ad-hoc: 
    the script user needs to specify the number of clients at each server through an array of length NServer called Rabia_ClientsPerServer

Rabia_ClientsPerServer:
  If "ad-hoc" is selected, the length of the array needs to be equal to the number of servers (NServers), 
  and the sum of entries needs to be equal to the number of clients (NClients).
  Otherwise (i.e., when "modulo" is selected), this array is disregarded.

NServers:             the number of servers (usually 2*NFaulty+1 or 4*NFaulty+1)
NFaulty:              the number of faulty servers (could be 0, 1, 2, ..., usually < floor((NServers-1)/2), in other words, f is less than the majority of servers)
NClients:             the number of clients (could be 1, 2, 3, ..., usually < 500)
NConcurrency:         concurrency -- the number of consensus instances (could be 1, 2, 3, ..., usually 1)
ClientTimeout:        the maximum time a client can run in seconds before it terminates (works only for closed-loop clients and has no effect on open-loop clients) (usually 60 or 180)
ClientThinkTime:      the time the client waits in between sending two requests in millisecond (ms) (could be 0, 1, 2, ..., usually 0)
ClientBatchSize:      the num. of write commands packed in a single client request (could be 1, 2, 3, ..., usually 1)
ProxyBatchSize:       the max. num. of client-batched requests in a consensus object (could be 1, 2, 3, ..., usually 1, 10, 100, 1000)
ProxyBatchTimeout:    (in milliseconds) a proxy sends a ConsensusObj when timed out or receives enough client-batched requests such that len(clientbatchedrequests) = proxybatchsize (could be 1, 2, 3, ..., usually 5)
NetworkBatchSize:     a reserved variable
NetworkBatchTimeout:  a reserved variable
NClientRequests:      if set to 0, then it becomes the default value 10000000 -- a very large number
                      if open-loop: the number of un-batched requests per client (1, 2, 3, ... usually 10000 or 100000);
                      if closed-loop: a client times out after reaching ClientTimeout time or receiving this many un-batched requests (usually 0 and use ClientTimeout to control the runtime)
END

# Section 1. user configurations - type 1 (see comments above for their meanings)
ServerIps=(localhost)
ClientIps=(localhost)
Controller=localhost:8070
ProxyStartPort=18080
NetworkStartPort=28080

User="$USER"
RCFolder=~/go/src/rabia

# Section 2. user configurations - type 2 (see comments above for their meanings)

RCLogLevel=debug
Rabia_ClosedLoop=true
Rabia_ClientDistributingMethod=modulo # ad-hoc | modulo
Rabia_ClientsPerServer=(1 0 0)

NServers=3
NFaulty=1
NClients=1
NConcurrency=1
ClientTimeout=20
ClientThinkTime=0
ClientBatchSize=1
ProxyBatchSize=1
ProxyBatchTimeout=10
NetworkBatchSize=0
NetworkBatchTimeout=0
NClientRequests=0

: <<'END'
    No need to modify variables and functions beyond this line 
END
# Section 3. runtime variables and functions
key=${RCFolder}/deployment/install/id_rsa
kill_sh=${RCFolder}/deployment/run/kill.sh
log_folder=${RCFolder}/logs
run_folder=${RCFolder}/deployment/run
# InitSoFar counts the number of clients initalized at each server so far;
# it is only used in ad-hoc mode and Rabia_ClientsPerServer is correctly provided.
InitSoFar=("${Rabia_ClientsPerServer[@]/*/0}") # initialize entires to zeros.

# loads SvrIps, SvrPPorts, SvrNPorts, RC_Proxies, and RC_Peers
load_variables_p1() {
    SvrIps=()
    SvrPPorts=()
    SvrNPorts=()
    RC_Proxies=()
    RC_Peers=()

    for idx in $(seq 0 $(($NServers - 1))); do
        SvrIpIdx=$((idx % ${#ServerIps[@]}))
        SvrIp=${ServerIps[$SvrIpIdx]}

        PPort=$((idx + ${ProxyStartPort}))
        NPort=$((idx + ${NetworkStartPort}))

        SvrIps+=(${SvrIp})
        SvrPPorts+=(${PPort})
        SvrNPorts+=(${NPort})
        RC_Proxies+=(${SvrIp}":"${PPort})
        RC_Peers+=(${SvrIp}":"${NPort})
    done

    # echo ${SvrIps[@]}
    # echo ${SvrPPorts[@]}
    # echo ${SvrNPorts[@]}
    # echo ${RC_Peers[@]}
    # echo ${RC_Proxies[@]}
    # echo ${CliIps[@]}
}

# loads Rabia_ClientsPerServer
load_variables_p2() {
    if [ "${Rabia_ClientDistributingMethod}" == "ad-hoc" ]; then
        sum=$(
            IFS=+
            echo "$((${Rabia_ClientsPerServer[*]}))"
        ) # sum up the number of clients per server specificed by user

        if [ ${#Rabia_ClientsPerServer[@]} -ne $NServers ]; then
            echo "ERROR: the length of Rabia_ClientsPerServer (${#Rabia_ClientsPerServer[@]}) is not equal to the parameter NServers ($NServers), exit now"
            return 1
        elif [ $sum -ne $NClients ]; then
            echo "ERROR: the sum of entries in Rabia_ClientsPerServer ($sum) is not equal to the parameter NClients ($NClients), exit now"
            return 1
        fi
    else
        Rabia_ClientsPerServer=()     # reset the array
        base=$((NClients / NServers)) # base = NClients // NServers
        rem=$((NClients % NServers))  # rem = NClients % NServers
        for idx in $(# for server indexed as idx:
            seq 0 $((NServers - 1))
        ); do
            if [ $idx -lt $rem ]; then                  #   if idx < rem:
                Rabia_ClientsPerServer+=($((base + 1))) #       append (base + 1) to the array Clients
            else                                        #   else:
                Rabia_ClientsPerServer+=($((base)))     #       append base to the array Clients
            fi
        done
    fi
    # echo ${Rabia_ClientDistributingMethod} "method for distrubting clients"
    # echo "num of clients taken by each server = "${Rabia_ClientsPerServer[*]}
}

# export 12 variables for go programs
export_variables() {
    export RC_Ctrl=${Controller} RC_Folder=${RCFolder} RC_LLevel=${RCLogLevel}
    export Rabia_ClosedLoop=${Rabia_ClosedLoop}

    export Rabia_NServers=$NServers Rabia_NFaulty=$NFaulty Rabia_NClients=$NClients Rabia_NConcurrency=$NConcurrency
    export Rabia_ClientBatchSize=$ClientBatchSize Rabia_ClientTimeout=$ClientTimeout Rabia_ClientThinkTime=$ClientThinkTime Rabia_ClientNRequests=$NClientRequests
    export Rabia_ProxyBatchSize=$ProxyBatchSize Rabia_ProxyBatchTimeout=$ProxyBatchTimeout Rabia_NetworkBatchSize=$NetworkBatchSize Rabia_NetworkBatchTimeout=$NetworkBatchTimeout
}

load_variables() {
    load_variables_p1 && load_variables_p2 && export_variables
    return $?
}

# $1: client index, finds the index of the proxy a client should connect to
find_proxy_idx() {
    local idx                                                    # to prevent conflict with the idx in start_clients
    if [ "${Rabia_ClientDistributingMethod}" == "ad-hoc" ]; then # if "ad-hoc":
        for idx in $(#   let idx be the index of the current entry (of array InitSoFar)
            seq 0 $((NServers - 1))
        ); do
            if [ ${InitSoFar[$idx]} -ne ${Rabia_ClientsPerServer[$idx]} ]; then # if InitSoFar[idx] != Rabia_ClientsPerServer[idx] (not enough clients are instantiated for this server):
                InitSoFar[$idx]=$((${InitSoFar[$idx]} + 1))                     #  increment InitSoFar[idx] to denote one client will be instantiated
                return $idx                                                     #  return idx -- idx-th server will be designated as this client's proxy
            fi
        done
        return 0                   # should never happen, because of the checks in calculate_vars
    else                           # if "modulo":
        return $(($1 % $NServers)) #    return client idx % num of servers
    fi
}

# function for both single.sh and multiple.sh

# remove the logs folder, the rabia binary, and result.txt
reset_folder() {
    rm -rf ${RCFolder}/logs
    rm -f ${RCFolder}/rabia
    rm -f ${RCFolder}/result.txt
}

build_binary() {
    PATH=${PATH}:/usr/local/go/bin          # for compiling over SSH
    GOPATH=~/go                             # for compiling over SSH
    go build -o ${RCFolder}/rabia ${RCFolder} # name the binary as rabia
    chmod +x ${RCFolder}/rabia                # for running over SSH
}

start_controller() {
    RC_Role=ctrl ${RCFolder}/rabia
}

reset_parameters() {
    source ${RCFolder}/deployment/profile/profile0.sh
}

# the following functions are for multiple.sh
start_servers_on_a_machine() {
    for idx in $(seq 0 $(($NServers - 1))); do
        ip_idx=$((idx % ${#ServerIps[@]}))
        if [[ ${ip_idx} -eq ${RCMachineIdx} ]]; then
            RC_Role=svr RC_Index=${idx} RC_SvrIp=${SvrIps[$idx]} RC_PPort=${SvrPPorts[$idx]} RC_NPort=${SvrNPorts[$idx]} RC_Peers=${RC_Peers[@]} ${RCFolder}/rabia &
        fi
    done
}

start_clients_on_a_machine() {
    for idx in $(seq 0 $(($NClients - 1))); do
        ip_idx=$((idx % ${#ClientIps[@]}))
        if [[ ${ip_idx} -eq ${RCMachineIdx} ]]; then
            find_proxy_idx ${idx}
            proxy_idx=$?
            RC_Role=cli RC_Index=${idx} RC_Proxy=${RC_Proxies[$proxy_idx]} ${RCFolder}/rabia &
        fi
    done
}

multi_reset_folder() {
    for ip in "${ServerIps[@]}"; do
        ssh -o StrictHostKeyChecking=no -i ${key} "${User}"@"${ip}" ". ${RCFolder}/deployment/profile/profile0.sh && reset_folder" &
    done
    for ip in "${ClientIps[@]}"; do
        ssh -o StrictHostKeyChecking=no -i ${key} "${User}"@"${ip}" ". ${RCFolder}/deployment/profile/profile0.sh && reset_folder" &
    done
    wait
}

multi_build_binary() {
    build_binary
    echo "distributing the built binary to all servers"
    for ip in "${ServerIps[@]}"; do
        scp -o StrictHostKeyChecking=no -i ${key} ${RCFolder}/rabia "${User}"@"${ip}":${RCFolder} &
    done
    for ip in "${ClientIps[@]}"; do
        scp -o StrictHostKeyChecking=no -i ${key} ${RCFolder}/rabia "${User}"@"${ip}":${RCFolder} &
    done
    wait
}

remove_logs() {
    for ip in "${ServerIps[@]}"; do
        ssh -o StrictHostKeyChecking=no -i ${key} "${User}"@"${ip}" "rm -rf ${log_folder}" &
    done
    for ip in "${ClientIps[@]}"; do
        ssh -o StrictHostKeyChecking=no -i ${key} "${User}"@"${ip}" "rm -rf ${log_folder}" &
    done
    wait
}

download_logs() {
    for ip in "${ServerIps[@]}"; do
        scp -o StrictHostKeyChecking=no -i ${key} "${User}"@"${ip}":${log_folder}/* ${log_folder} &
    done
    for ip in "${ClientIps[@]}"; do
        scp -o StrictHostKeyChecking=no -i ${key} "${User}"@"${ip}":${log_folder}/* ${log_folder} &
    done
    wait
}
