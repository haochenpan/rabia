# Install and Run Sync-Rep (Synchronous replication using standalone Redis)

Code: [Link to repository of redis-sync-rep](https://github.com/YichengShen/redis-sync-rep)

## 2 Configurations
- (i) Sync-Rep (1): two VMs, one as the Redis leader and the other as a follower; and 
- (ii) Sync-Rep (2): three VMs, one as the Redis leader and the other two as followers.
- Note: We run clients on follower VMs to force the system to have one RTT so that it’s compatible with SMR-based approach.

## To Run

1. Start VMs
    - Start appropriate number of VMs according to Sync-Rep (1) or (2).
    - OS Assumption: Ubuntu 16.04

2. On each VM, complete the following steps:

    - Clone [Rabia](https://github.com/haochenpan/rabia) repository
        ```shell
        sudo su
        mkdir -p ~/go/src && cd ~/go/src
        git clone https://github.com/haochenpan/rabia.git
        ```

    - Install Rabia and its dependencies (Dependencies of Sync-Rep are included in Rabia's installation script.)
        ```shell
        cd ./rabia/deployment
        . ./install/install.sh
        ```

    - Clone Sync-Rep repository
        ```shell
        cd ~/go/src
        git clone https://github.com/YichengShen/redis-sync-rep.git
        cd redis-sync-rep
        ```

    - Configure IP of master VM
        - In `config.yaml`, change 'MasterIp' to the IP of your master VM.

    - Start Redis server: You could configure the current VM either as a master or a replica.
        - configure as master
            ```shell
            . ./deployment/startRedis/startServer.sh
            ```
        - configure as replica
            ```shell
            . ./deployment/startRedis/startServer.sh replica
            ```
        - Note: if run successfully, you should see     
            OK      
            PONG
   
3. Run Sync-Rep     
    - To produce the lower bar:
        - Change the following parameters in `config.yaml`      
            ```yaml
            NClients: 1
            ClientBatchSize: 1
            ```
        - On one of the follower VMs, run the main program
            ```shell
            go run main.go
            ```
    - To produce the higher bar:
        - Change the following parameters in `config.yaml` 
            ```yaml
            NClients: 15 
            ClientBatchSize: 20 
            ```
        - On one of the follower VMs, run the main program
            ```shell
            go run main.go
            ```

