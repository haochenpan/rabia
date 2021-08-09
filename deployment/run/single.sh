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
END
single_start_servers() {
  for idx in $(seq 0 $(($NServers - 1))); do
    RC_Role=svr RC_Index=${idx} RC_SvrIp=${SvrIps[$idx]} RC_PPort=${SvrPPorts[$idx]} RC_NPort=${SvrNPorts[$idx]} RC_Peers=${RC_Peers[@]} ${RCFolder}/rabia &
  done
}

single_start_clients() {
  for idx in $(seq 0 $(($NClients - 1))); do
    find_proxy_idx ${idx}
    proxy_idx=$?
    RC_Role=cli RC_Index=${idx} RC_Proxy=${RC_Proxies[$proxy_idx]} ${RCFolder}/rabia &
  done
}

# benchmark the cluster with provided configurations once and analyze performance statistics
run_once() {
  # 0. remove previous logs
  rm -rf ${RCFolder}/logs/
  # 1. load Rabia extra profiles and variables
  if [ $# -ne 0 ]; then # if there's a profile passed in
    . ${1}              # load it
  fi
  load_variables
  if [ $? -ne 0 ]; then # if there's an error
    return 1            # early exit
  fi
  # 2. build Rabia binary
  build_binary
  # 3. start all servers
  single_start_servers
  # 4. start all clients
  single_start_clients
  # 5. start the controller and wait its return
  start_controller
  # 6. analysis the generated logs
  # available and optional flags: print-title, print-round-dist
  python3.8 ${RCFolder}/deployment/analysis/analysis.py ${RCFolder}/logs print-title print-round-dist 1>>${RCFolder}/result.txt

  # 7. reset shell variables (for the next run)
  reset_parameters
}

# visual benchmark: run this function and then check result.txt to see
# whether Rabia works on a single machine well with specified parameters
visual_benchmark() {
  reset_folder

  echo "visual benchmark -- start" >>${RCFolder}/result.txt
  echo "1. use the default configurations in profile.sh"
  run_once

  echo "2. change the number of servers"
  NServers=5
  run_once

  echo "3. change the numbers of servers and faulty servers"
  NServers=5
  NFaulty=2
  run_once

  echo "4. change the number of clients"
  NClients=5
  run_once

  echo "5. change the client batch size"
  ClientBatchSize=10
  run_once

  echo "6. change the proxy batch size"
  NClients=9
  ProxyBatchSize=3
  run_once

  echo "visual benchmark -- end" >>${RCFolder}/result.txt
}

source ../profile/profile0.sh # required, load the variables and functions
run_once # run with the default parameters in profile0.sh
# visual_benchmark # run with the default parameters in profile0.sh PLUS a few quick change of parameters
