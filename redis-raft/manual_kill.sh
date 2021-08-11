ROOT_FOLDER=~/go/src/rabia/
RR_FOLDER=${ROOT_FOLDER}/redis-raft
SERVER_IPS=( 10.10.1.1 10.10.1.3 10.10.1.5 )
SSH_KEY=${ROOT_FOLDER}/deployment/install/id_rsa
BASE_PORT=5001

function kill_all_servers(){
  i=0
  for ip in "${SERVER_IPS[@]}"; do
      echo "INFO: Killing server $((i + 1)) at IP: ${ip}..."
      ssh -o StrictHostKeyChecking=no -i ${SSH_KEY} root@"$ip" "kill -9 ${BASE_PORT}; lsof -ti tcp:${BASE_PORT} | xargs kill & 2>&1; sudo service redis-server stop & 2>&1; ps -ef |grep redis; cd ${RR_FOLDER} && . clean.sh"
      echo "SUCCESS: Killed server $((i + 1))"
      ((i++))
  done
}
kill_all_servers