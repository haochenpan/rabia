# step 0. adjust ServerIps, ClientIps, and Controller in profile0.sh (see Section 4 of docs/run-rabia.md),
#         and adjust other  Section 1 settings in profile0.sh if needed.

# step 1. run the configuration below through multiple.sh

# note: 4a, 4b, 4d: servers are instantiated in the same availability zone,
#       4c: servers are instantiated in different availability zones (one in us-east-1-b, one in us-east-1-c, and the last in us-east-1-d)

RCLogLevel=warn
Rabia_ClosedLoop=true
Rabia_ClientDistributingMethod=modulo

NServers=5
NFaulty=2
NClients=20 # 40, 60, 80, 100, 200, 300, 400, 500
NConcurrency=1
ClientTimeout=120
ClientThinkTime=0
ClientBatchSize=10
ProxyBatchSize=20
ProxyBatchTimeout=5000000
NClientRequests=0



