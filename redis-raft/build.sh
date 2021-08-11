RedisRaftRootFolder=~/redisraft

function build_project(){
  echo "Starting Build..."
  cd "${RedisRaftRootFolder}" || return

  git submodule init
  git submodule update
  make -B

  echo "Finished Build..."
  cd "${RedisRaftRootFolder}" || return
}
build_project