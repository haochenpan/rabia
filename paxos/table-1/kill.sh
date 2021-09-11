pgrep -af /root/go/src/rabia/paxos/table-1/bin/master | while read -r pid cmd ; do
     echo "pid: $pid, cmd: $cmd"
    kill -9 $pid > /dev/null 2>&1
done
pgrep -af /root/go/src/rabia/paxos/table-1/bin/server | while read -r pid cmd ; do
     echo "pid: $pid, cmd: $cmd"
    kill -9 $pid > /dev/null 2>&1
done
pgrep -af /root/go/src/rabia/paxos/table-1/bin/client | while read -r pid cmd ; do
     echo "pid: $pid, cmd: $cmd"
    kill -9 $pid > /dev/null 2>&1
done