EPaxosEnabled=true
MASTER_SERVER_IP="10.10.1.1"

bin/master -N 3 &
sleep 0.1
bin/server -maddr ${MASTER_SERVER_IP} -addr ${MASTER_SERVER_IP} -e=${EPaxosEnabled} &
