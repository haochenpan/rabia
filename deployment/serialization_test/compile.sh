: <<'END'
    Copyright 2021 Rabia Research Team and Developers

    Licensed under the Apache License, Version 2.0 (the "License");
    you may not use this file except in compliance with the License.
    You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

    Unless required by applicable law or agreed to in writing, software
    distributed under the License is distributed on an "AS IS" BASIS,
    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    See the License for the specific language governing permissions and
    limitations under the License.
END
# call this script inside the serialization folder
# serialization folder: local testing
# serialization_test folder: cluster testing

cd serialization
rm -f *.pb.go
~/go/src/gobin-codegen/bin/bi ~/go/src/rc3/deployment/serialization_test/serialization/gobin_msg.go \
    > ~/go/src/rc3/deployment/serialization_test/serialization/gobin_msg.pb.go
protoc -I=. -I=$GOPATH/src -I=$GOPATH/src/google/protobuf --go_out=. ./proto_msg.proto
protoc -I=. -I=$GOPATH/src -I=$GOPATH/src/github.com/gogo/protobuf/protobuf --gogoslick_out=. ./gogo_msg.proto
cd ..

