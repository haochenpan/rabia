source ./base-profile.sh

pgrep -af ${EPaxosFolder}/bin/master | while read -r pid cmd ; do
     echo "pid: $pid, cmd: $cmd"
    kill -9 $pid > /dev/null 2>&1
done
pgrep -af ${EPaxosFolder}/bin/server | while read -r pid cmd ; do
     echo "pid: $pid, cmd: $cmd"
    kill -9 $pid > /dev/null 2>&1
done
pgrep -af ${EPaxosFolder}/bin/client | while read -r pid cmd ; do
     echo "pid: $pid, cmd: $cmd"
    kill -9 $pid > /dev/null 2>&1
done