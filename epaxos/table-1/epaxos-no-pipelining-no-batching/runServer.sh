EPaxosEnabled=true
MASTER_SERVER_IP="10.10.1.1"
REPLICA_SERVER_IP="10.10.1.2"

bin/server -maddr ${MASTER_SERVER_IP} -addr ${REPLICA_SERVER_IP} -e=${EPaxosEnabled} &