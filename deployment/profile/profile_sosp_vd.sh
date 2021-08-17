# step 0. adjust ServerIps, ClientIps, and Controller in profile0.sh (see Section 4 of docs/run-rabia.md),
#         and adjust other  Section 1 settings in profile0.sh if needed.

# step 1. run the configuration below through multiple.sh

RCLogLevel=warn
Rabia_ClosedLoop=false
Rabia_ClientDistributingMethod=modulo

NServers=3
NFaulty=1
NClients=15
NConcurrency=1
ClientTimeout=120
ClientThinkTime=0
ClientBatchSize=20
ProxyBatchSize=15
ProxyBatchTimeout=5000000
NClientRequests=1000