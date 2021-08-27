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

# install.sh installs Rabia and copies a SSH public key to a fresh VM so that the benchmark controller server
# could use Shell scripts to control this VM.
# Remove the corresponding function calls at the bottom if Rabia is not installed on a fresh machine.
# OS assumption: Ubuntu 16.04, Ubuntu 18.04
# Root access are required for some operations during installation, see commands prefixed with "sudo."
# However, Rabia does not need to go to /root/go/src, instead, it can sits inside some non-root user's ~/go/src folder
# Rabia runs does not require the root access

rabia_folder=~/go/src/rabia        # the path to the Rabia folder
redis_folder=~                     # the path where Redis is installed
go_tar=go1.15.8.linux-amd64.tar.gz # the version of Golang to be downloaded in install_go
py_ver=python3.8                   # the version of Python to be downloaded in install_python
redis_ver=redis-6.2.2

# Copies the SSH public key to the "authorized_keys" file so that the benchmark controller could control this VM
function install_key() {
    mkdir -p ~/.ssh/
    cat "${rabia_folder}"/deployment/install/id_rsa.pub >>~/.ssh/authorized_keys
    chmod 400 "${rabia_folder}"/deployment/install/id_rsa
}

# Installs a version of Python, which is used by the analysis program
function install_python() {
    sudo apt update
    sudo apt install -y gcc
    sudo apt install -y software-properties-common
    sudo add-apt-repository -y ppa:deadsnakes/ppa
    sudo apt update
    sudo apt install -y ${py_ver}
    ${py_ver} --version
}

# Installs a version of Golang since Rabia is written in Golang
function install_go() {
    wget -q https://golang.org/dl/${go_tar}
    sudo tar -C /usr/local -xzf ${go_tar}
    rm ${go_tar}
    echo 'export PATH=${PATH}:/usr/local/go/bin' >>~/.bashrc
    echo 'export GOPATH=~/go' >>~/.bashrc
    echo 'export GO111MODULE="auto"' >>~/.bashrc
    source ~/.bashrc
    go version
}

# Installs pip and numpy for python3. Used for non-pipelined Paxos and EPaxos testing.
function install_numpy() {
    sudo apt install -y python3.8-distutils
    wget https://bootstrap.pypa.io/get-pip.py
    sudo python3.8 get-pip.py
    pip install numpy
}

# Installs some Golang packages used by Rabia
function install_go_deps() {
    go get -u github.com/gogo/protobuf/gogoproto           # gogo-protobuf
    go get -u google.golang.org/protobuf/cmd/protoc-gen-go # vanilla protobuf
    go get -u github.com/rs/zerolog/log
    go get -u github.com/go-redis/redis/v8
}

function install_redis() {
    cd ${redis_folder}
    sudo apt install -y tar make
    wget https://download.redis.io/releases/${redis_ver}.tar.gz
    tar xzf ${redis_ver}.tar.gz
    rm ${redis_ver}.tar.gz
    cd ${redis_ver}
    make
    cd ~/go/src/rabia/deployment
}

install_key
install_python
install_numpy
install_go
install_go_deps
install_redis
