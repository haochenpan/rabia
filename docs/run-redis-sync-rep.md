# Install and Run Redis Synchronous Replication

Code: [Link to repository of redis-sync-rep](https://github.com/YichengShen/redis-sync-rep)

## 1. Start new VMs

The number of VMs depends on your setup. The minimum is 2 VMs (one being the master and the other being a replica).

## 2. On each VM, complete the following steps:

### 2.1 Clone the repository
```shell
sudo su && cd ~
mkdir -p ~/go/src
cd ~/go/src
git clone https://github.com/YichengShen/redis-sync-rep.git
cd redis-sync-rep
```

### 2.2 Install Redis, Go, and dependancies
```shell
. ./deployment/install/install.sh
```

### 2.3 Start Redis server
You could configure the current VM either as a master or a replica.
- configure as a master
    ```shell
    . ./deployment/startRedis/startServer.sh
    ```
- configure as a replica
    ```shell
    . ./deployment/startRedis/startServer.sh replica
    ```

Note: For the minimum requirement, you need 2 VMs. You configure one to be the master and the other to be a replica using the commands above.
    
### 2.4 Adjust parameters 
The configuration file is `config.yaml`. See comments for the meaning of parameters.

## 3. From one of the client VMs, run the main program
```shell
go run main.go
```
The program will run according to the configuration file and print out the results which will also be saved in the logs folder.  