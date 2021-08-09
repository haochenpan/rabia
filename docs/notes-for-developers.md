# Notes for Developers

## FAQ

#### Q: why there are *-0.log and *-1.log files generated in the logs folder?

A: A Rabia component (e.g., a Rabia client, a proxy layer, a network layer, or a consensus layer) may use more than one
loggers, their generated files are indexed by -0, -1, -2, etc.

#### Q: why the codebase has many Shell/bash files, I thought this is a Go project:

A: For faster deployment and testing. The purpose of shell files is you don’t have to start one server/client after 
another on a cluster of VMs. A shell function call does all for you. Nevertheless, please feel free to mimic the
commands in Shell scripts to start a Rabia cluster manually. 

## Miscellaneous things

### A problem of GCP

If we run `single.sh` or `multiple.sh` on GCP's web terminal over SSH directly, sometimes the output will be truncated
in the middle: you may see a bunch of lines that produced initially and a few lines that are recently produced, but the 
lines between them  are missing. A solution is to run `single.sh` or `multiple.sh` on your desktop's terminal, i.e., 
connect to the controller server (server-0) through the project-wide SSH key from your developer-machine's terminal app 
and run the starting Shell script there.

### How to upload files from your developer machine to a cluster of VMs

- method 1: through `scp`
- method 2: through whatever tools you like, e.g. XShell
- method 3:  fork this repository, update the changed files, and `git clone` this repository on each VM
- method 4: through GoLand->Preferences->Build, Execution, Deployment->Deployment:
    - for each VM, add a SFTP entry. For example
        - name: server-0
        - SSH configurations:
            - Host: <server-0's public IP>
            - Port: 22
            - User name: <first-server-VM-user-name>, e.g., root, myName_myInstitution_[com | edu | org]
            - Authentication type: "Key pair"
            - Private key file: <developer-machine's-path-to-Rabia-project>/rc3/deployment/install/id_rsa
            - Passphrase: (empty)
            - hit "Test connection"
        - Root path: (default)
        - Web server URL: (default)
        - go to the "Mapping" tab:
            - Local path: <developer-machine's-path-to-Rabia-project>, e.g., /Users/roger/go/src/rc3
            - Deployment path: /home/<first-server-VM-user-name>/go/src/rc3 (for non-root user), or /root/go/src/rc3 for
              the root user
            - Web path: /
    - add a "server group," place all VMs to this group
    - select modified file(s) (i.e., `profile0.sh`) in the Project View, then go to Tools->Deployment->"Upload to .."
    - select the server group to selected file(s) to every server in that group
    - it is highly recommended checking whether the file has really been updated to all servers at least once after you
      start/reboot VMs. I often mis-configure "SSH configurations" part of each VM since I use the 2021 version of
      GoLand. Somehow, the "autodetect"-ed "Root path" may let GoLand upload your stuff to a wrong folder even you set
      Mapping correctly. So don't hit the "autodetect" button ever.
    - other resources see [here](https://www.jetbrains.com/help/go/deploying-applications.html).

    > Important: if you installed Rabia in a non-root folder on the VM (e.g., /home/userX/go/src), in Goland, 
    you should provide userX as the SSH connection username. Moreover, make sure the path mapping indeed goes to the 
    rabia folder in /home/userX/go/src.

### Vanilla protobuf vs. gogo-protobuf

This branch currently uses gogo-protobuf, to test it under vanilla protobuf, do the following:

- in `msg.proto`, active the "vanilla protobuf header" and deactivate other protobuf headers
- recompile `msg.proto` according to the comments at the file header
- in `msg.go`, active "vanilla protobuf variables", uncomment four lines with "vanilla protobuf" notes, and comment out
  their counterparts.
- in `network.go`, active uncomment two lines with "vanilla protobuf" notes, and comment out their counterparts.

### Avoid remote building the go binary

Some env variables are not the same when you type `go build` on a VM and when your program SSH to the remote VM. That
results to failures in building the binary. Now the solution: build on the controller machine and scp to all remote
machines.

### Install NetData to monitor the system resources

Note: it is better to install NetData and claim nodes AFTER you have spawned all VMs you needed:
I tried to install Rabia and NetData on a single machine, and use the image of this machine to spawn a cluster of VMs;
Effectively, I install NetData BEFORE I spawned all VMs I needed. However, I was not able to claim these nodes in my War
Room as they are treated as a single node by NetData. I am sure there are some solution to this.

The following commands install NetData to your VM:

```bash
sudo su
bash <(curl -Ss https://my-netdata.io/kickstart.sh)
```

After NetData is installed, go to the Netdata Cloud Website (sign-in required), create a war room if necessary.

Next, go to "Manage War Room / Add Nodes." copy the command there and paste to the server's terminal to claim the node.

Switch back the NetData website so monitor the node's resources.
