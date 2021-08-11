# This script is intended for downloading and setting up Redis and C compilation on Ubuntu
# This also assumes adequate version of C installed already

ROOT_FOLDER=~/go/src/rabia/redis-raft

function install_c_make(){
  sudo apt-get install build-essential libssl-dev
  wait
  cd /tmp
  wget https://github.com/Kitware/CMake/releases/download/v3.20.0/cmake-3.20.0.tar.gz
  wait
  tar -zxvf cmake-3.20.0.tar.gz
  wait
  cd cmake-3.20.0
  ./bootstrap
  wait
  make
  sudo make install
  wait
  cmake --version
}

function install_gnu_autotooling(){
  sudo apt-get install -y autotools-dev # install autotooling
  sudo apt-get install -y autoconf # install autoconf
  sudo apt-get install -y libtool # install libtool
}

function get_lib_bsd(){
  sudo apt-get install -y libbsd-dev #use bsd queue.h implementation
}

# Note that if you have Rabia installed on current machine, a new redis version will be downloaded
# in addition to more packages required for redis-raft

function install_redis(){
  sudo add-apt-repository ppa:redislabs/redis
  sudo apt-get install -y redis
  sudo apt install redis-server
  sudo systemctl status redis
}

function main(){
  echo "Starting installation..."
  sudo apt-get update #update apt for latest package -vs
  install_c_make # need cmake for compiling src
  install_gnu_autotooling
  get_lib_bsd
  install_redis
  echo "Finished installation..."
  cd ${ROOT_FOLDER}
}

main