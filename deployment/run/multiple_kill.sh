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
kill_all() {
  for ip in "${ServerIps[@]}"; do
    ssh -o StrictHostKeyChecking=no -i ${key} "${User}"@"${ip}" ". ${kill_sh}" 2>&1 &
  done
  for ip in "${ClientIps[@]}"; do
    ssh -o StrictHostKeyChecking=no -i ${key} "${User}"@"${ip}" ". ${kill_sh}" 2>&1 &
  done
}

# must be called at this folder, e.g., do sth like cd .../deployment/run && . <file name>.sh
source ../profile/profile0.sh
kill_all
