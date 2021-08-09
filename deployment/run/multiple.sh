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
start_servers() {
  MachineIdx=0
  for ip in "${ServerIps[@]}"; do
    if [ $# -eq 0 ]; then
      ssh -o StrictHostKeyChecking=no -i ${key} "${User}"@"${ip}" ". ${RCFolder}/deployment/profile/profile0.sh && load_variables && RCMachineIdx=${MachineIdx} start_servers_on_a_machine" 2>&1 &
    else
      ssh -o StrictHostKeyChecking=no -i ${key} "${User}"@"${ip}" ". ${RCFolder}/deployment/profile/profile0.sh && . ${1} && load_variables && RCMachineIdx=${MachineIdx} start_servers_on_a_machine" 2>&1 &
    fi
    ((MachineIdx++))
  done
}

start_clients() {
  MachineIdx=0
  for ip in "${ClientIps[@]}"; do
    if [ $# -eq 0 ]; then
      ssh -o StrictHostKeyChecking=no -i ${key} "${User}"@"${ip}" ". ${RCFolder}/deployment/profile/profile0.sh && load_variables && RCMachineIdx=${MachineIdx} start_clients_on_a_machine" 2>&1 &
    else
      ssh -o StrictHostKeyChecking=no -i ${key} "${User}"@"${ip}" ". ${RCFolder}/deployment/profile/profile0.sh && . ${1} && load_variables && RCMachineIdx=${MachineIdx} start_clients_on_a_machine" 2>&1 &
    fi
    ((MachineIdx++))
  done
}

# benchmark the cluster with provided configurations once, download logs, and analyze performance statistics
run_once() {
  chmod 400 ${RCFolder}/deployment/install/id_rsa
  # 0. remove previous logs
  remove_logs
  # 1. load Rabia extra profiles and variables
  if [ $# -ne 0 ]; then # if there's a profile passed in
    . ${1}              # load it
  fi
  load_variables
  if [ $? -ne 0 ]; then # if there's an error
    return 1            # early exit
  fi
  echo ${Rabia_ClientDistributingMethod} "method for distrubting clients"
  echo "num of clients taken by each server = "${Rabia_ClientsPerServer[*]}
  # 2. build Rabia binary
  multi_build_binary
  # 3. start all servers
  start_servers ${1}
  # 4. start all clients
  start_clients ${1}
  # 5. start the controller and wait its return
  start_controller
  # 6. analysis the generated logs
  download_logs
  # available and optional flags: print-title, print-round-dist
  python3.8 ${RCFolder}/deployment/analysis/analysis.py ${RCFolder}/logs print-round-dist 1>>${RCFolder}/result.txt
  # 7. reset shell variables (for the next run)
  reset_parameters
}

# visual benchmark: run this function and then check result.txt to see
# whether Rabia works on a single machine well with specified parameters
visual_benchmark() {
  multi_reset_folder

  echo "visual benchmark -- start" >>${RCFolder}/result.txt
  echo "1. use the default configurations in profile.sh"
  run_once

  echo "2. base profile (profile0.sh) and profile1.sh"
  run_once ${RCFolder}/deployment/profile/profile1.sh

  echo "visual benchmark -- end" >>${RCFolder}/result.txt
}

cluster_run() {
  multi_reset_folder

  run_once ${RCFolder}/deployment/profile/profile0.sh

  run_once ${RCFolder}/deployment/profile/profile1.sh

}

source ../profile/profile0.sh # required, load the variables and functions
run_once
# visual_benchmark
# cluster_run
