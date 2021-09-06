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
# Disable all CPUs except cpu 0 and 8 (the first core), and 1 and 9 (the second core)
# modify numbers in cpu[a-b] to control number of CPUs to disable
for cpu in {2..7}
do
    echo 0 > /sys/devices/system/cpu/cpu$cpu/online
done

for cpu in {10..15}
do
    echo 0 > /sys/devices/system/cpu/cpu$cpu/online
done
