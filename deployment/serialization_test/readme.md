Compare the serialization performance of [Gobin](https://github.com/efficient/gobin-codegen), 
[Protobuf](https://github.com/protocolbuffers/protobuf), and 
[GoGo-Protobuf](https://github.com/gogo/protobuf)

1. Install Gobin:
```shell
cd go/src
git clone https://github.com/efficient/gobin-codegen.git
cd gobin-codegen
export GOPATH=`/bin/pwd`
go install bi
# log out your terminal and log in again to restore the GOPATH
```

2. Install Protobuf and GoGo-Protobuf: see comments in `rabia/internal/msg/msg.proto`

3. serialization_test/serialization:

- for running **local tests**
- remove 6 gobin*, gogo*, proto* files
- comment out every line of local_serialization_test.go and setup.go, except their first lines
- modify KeyNum in struct_gen_test.go, and run it to generate structs
- uncomment all previous commented lines in two files
- make sure readBufSize in local_serialization_test.go is enough for your data, so as various variables in
  GetReaderWriter in setup.go (they should handle a few KBs messages sizes)
- call `. run.sh`

4. serialization_test: cluster tests

- for running tests **between two VMs**
- do the steps described in local tests to make sure your message struct works
- change variable at the head of `serialization.go`
- call `compile.sh` on the developer machine
- upload files to two VMs
- VM0: `go run serialization.go -idx=0`
- VM1: `go run serialization.go -idx=1`


    