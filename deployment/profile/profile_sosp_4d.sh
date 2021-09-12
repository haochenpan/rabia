# step 0. adjust ServerIps, ClientIps, and Controller in profile0.sh (see Section 4 of docs/run-rabia.md),
#         and adjust other  Section 1 settings in profile0.sh if needed.

# step 1. run the configuration below through multiple.sh

# note: 4a, 4b, 4d: servers are instantiated in the same availability zone,
#       4c: servers are instantiated in different availability zones (one in us-east-1-b, one in us-east-1-c, and the last in us-east-1-d)

RCLogLevel=warn
Rabia_ClosedLoop=true
Rabia_ClientDistributingMethod=ad-hoc

NServers=5
NFaulty=2
NConcurrency=1
ClientTimeout=120
ClientThinkTime=0
ClientBatchSize=10
ProxyBatchSize=20
ProxyBatchTimeout=5000000
NClientRequests=0


# 20 clients, comments out other 2-line blocks except this one
NClients=20
Rabia_ClientsPerServer=(20 0 0)

# 40 clients, comments out other 2-line blocks except this one
# NClients=40
# Rabia_ClientsPerServer=(20 20 0)

# 60 clients, comments out other 2-line blocks except this one
# NClients=60
# Rabia_ClientsPerServer=(20 20 20)

# 80 clients, comments out other 2-line blocks except this one
# NClients=80
# Rabia_ClientsPerServer=(40 20 20)

# 100 clients, comments out other 2-line blocks except this one
# NClients=100
# Rabia_ClientsPerServer=(40 40 20)

# 200 clients, comments out other 2-line blocks except this one
# NClients=200
# Rabia_ClientsPerServer=(80 60 60)

# 300 clients, comments out other 2-line blocks except this one
# NClients=300
# Rabia_ClientsPerServer=(100 100 100)

# 400 clients, comments out other 2-line blocks except this one
# NClients=400
# Rabia_ClientsPerServer=(140 140 120)

# 500 clients, comments out other 2-line blocks except this one
# NClients=500
# Rabia_ClientsPerServer=(180 160 160)



