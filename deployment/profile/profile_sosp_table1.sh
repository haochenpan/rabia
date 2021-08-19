# step 0. adjust ServerIps, ClientIps, and Controller in profile0.sh (see Section 4 of docs/run-rabia.md),
#         and adjust other  Section 1 settings in profile0.sh if needed.

# step 1. run the configuration below through multiple.sh

RCLogLevel=warn
Rabia_ClosedLoop=true
Rabia_ClientDistributingMethod=modulo

NServers=3
NFaulty=1
NClients=2 # if the median latency is too large, try 20 clients or 5 clients
NConcurrency=1
ClientTimeout=60
ClientThinkTime=0
ClientBatchSize=1
ProxyBatchSize=1
ProxyBatchTimeout=5
NClientRequests=0





