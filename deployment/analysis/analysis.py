"""
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
"""
import concurrent.futures
import os
from json import load, loads
from os import listdir
from os.path import isfile, join
from sys import argv
from collections import defaultdict


def get_experiments(log_folder_path):
    files = [f for f in listdir(log_folder_path) if isfile(join(log_folder_path, f)) and "log" in f]
    params = set()
    for file in files:
        params.add(file[:file.find("--")])
    # print(params)
    return sorted(params)


def load_json_dict(log_file_path):
    with open(log_file_path) as f:
        return load(f)


def load_line_file(log_file_path):
    with open(log_file_path) as f:
        return f.readlines()


def load_client_info(log_folder, param, trial):
    for i in range(trial["nc"]["v"]):
        client_log_file_name = f"{param}--client-{i}-0.log"
        client_log_dict = load_json_dict(os.path.join(log_folder, client_log_file_name))
        # print(client_log_dict) ##
        trial["client_log_dict_list"].append(client_log_dict)
        trial["client_send_time_list"].append((client_log_dict["sendEnd"] - client_log_dict["sendStart"]))
        trial["client_recv_time_list"].append((client_log_dict["recvEnd"] - client_log_dict["sendStart"]))
        trial["mid80RecvTimeDurs"].append((client_log_dict["mid80RecvTimeDur"]))  # in seconds

    minLat = min([d["minLat"] for d in trial["client_log_dict_list"]])
    maxLat = max([d["maxLat"] for d in trial["client_log_dict_list"]])
    avgLat = sum([d["avgLat"] for d in trial["client_log_dict_list"]]) / len(trial["client_log_dict_list"])
    p50Lat = sum([d["p50Lat"] for d in trial["client_log_dict_list"]]) / len(trial["client_log_dict_list"])
    p95Lat = sum([d["p95Lat"] for d in trial["client_log_dict_list"]]) / len(trial["client_log_dict_list"])
    p99Lat = sum([d["p99Lat"] for d in trial["client_log_dict_list"]]) / len(trial["client_log_dict_list"])
    trial["minLat"] = {"p": "minimum latency (ms)", "v": round(minLat / 10 ** 3, 2)}
    trial["maxLat"] = {"p": "maximum latency (ms)", "v": round(maxLat / 10 ** 3, 2)}
    trial["avgLat"] = {"p": "average latency (ms)", "v": round(avgLat / 10 ** 3, 2)}
    trial["p50Lat"] = {"p": "50 percentile latency (ms)", "v": round(p50Lat / 10 ** 3, 2)}
    trial["p95Lat"] = {"p": "95 percentile latency (ms)", "v": round(p95Lat / 10 ** 3, 2)}
    trial["p99Lat"] = {"p": "99 percentile latency (ms)", "v": round(p99Lat / 10 ** 3, 2)}

    avgSendTime = sum(trial["client_send_time_list"]) / len(trial["client_send_time_list"])
    avgRecvTime = sum(trial["client_recv_time_list"]) / len(trial["client_recv_time_list"])
    maxSendTime = max(trial["client_send_time_list"])
    maxRecvTime = max(trial["client_recv_time_list"])
    trial["avgSendTime"] = {"p": "average client send time (sec)", "v": round(avgSendTime / 10 ** 9, 2)}
    trial["avgRecvTime"] = {"p": "average client receive time (sec)", "v": round(avgRecvTime / 10 ** 9, 2)}
    trial["maxSendTime"] = {"p": "maximum client send time (sec)", "v": round(maxSendTime / 10 ** 9, 2)}
    trial["maxRecvTime"] = {"p": "maximum client receive time (sec)", "v": round(maxRecvTime / 10 ** 9, 2)}

    # exclude the heads and tails for throughput & latency reporting, which is a common practice
    mid80RecvTime = [d["mid80End"] - d["mid80Start"] for d in trial["client_log_dict_list"]]  # in ns
    # print(mid80RecvTime)
    mid80AvgRecvTime = round((sum(mid80RecvTime) / len(mid80RecvTime)) / 10 ** 9, 2)
    mid80MaxRecvTime = round(max(mid80RecvTime) / 10 ** 9, 2)
    mid80SumRequests = sum([d["mid80Requests"] for d in trial["client_log_dict_list"]])
    mid80Throughput = round(mid80SumRequests / mid80MaxRecvTime, 2)
    trial["mid80AvgRecvTime"] = {"p": "average client receive time -- clients middle 80% (sec)", "v": mid80AvgRecvTime}
    trial["max80MaxRecvTime"] = {
        "p": "maximum client receive time -- clients middle 80% (sec) (first send to last recv)", "v": mid80MaxRecvTime}
    trial["mid80SumRequests"] = {"p": "number of client requests -- clients middle 80%", "v": mid80SumRequests}
    trial["mid80Throughout"] = {"p": "throughput 1 (ops/sec) -- clients middle 80% (sec) (first send to last recv)", "v": mid80Throughput}

    # for open-loop saturation test
    mid80MaxRecvToRecvTime = max(trial["mid80RecvTimeDurs"])  # in sec
    mid80Throughput2 = round(mid80SumRequests / mid80MaxRecvToRecvTime, 2)
    trial["max80MaxRecvTime 2"] = {
        "p": "maximum client receive time -- clients middle 80% (sec) (last send to last recv)", "v": mid80MaxRecvTime}
    trial["mid80Throughout 2"] = {"p": "throughput 2 (ops/sec) -- clients middle 80% (sec) (last send to last recv)", "v": mid80Throughput2}


def load_proxy_info(log_folder, param, trial):
    with concurrent.futures.ThreadPoolExecutor() as executor:
        futures = []
        for i in range(trial["ns"]["v"]):
            proxy_log_file_name = f"{param}--proxy-{i}-0.log"
            proxy_log_file_path = os.path.join(log_folder, proxy_log_file_name)
            futures.append(executor.submit(load_line_file, proxy_log_file_path))
        futures_res = [f.result() for f in futures]
        # sometimes, a server exits a little bit early so the last a few lines are not written to logs
        # so we compare the prefix of logs, which should entail > 99% of the runtime
        min_len = min([len(l) for l in futures_res])
        for res in futures_res:
            assert res[:min_len] == futures_res[0][:min_len], "server data store dictionaries are not equal"
    trial["correctness-proxy"] = {"p": "proxy level file check passed", "v": True}


def load_consensus_info(log_folder, param, trial):
    con_ins_num_of_nor_mis_slots = []
    con_ins_num_of_total_slots = []

    avg_num_of_rounds = []
    p95_num_of_rounds = []
    p99_num_of_rounds = []
    max_num_of_rounds = []
    round_dist_arrays = []
    for c in range(trial["nC"]["v"]):  # for each consensus instance
        server_list = []
        for s in range(trial["ns"]["v"]):  # for each server
            log_file_name = f"{param}--consensus-{s}-{c}.log"
            log_dict = load_json_dict(os.path.join(log_folder, log_file_name))
            # print(log_dict) ##
            server_list.append(log_dict)
        assert len(server_list) == trial["ns"]["v"]

        first = server_list[0]
        this_normal_and_mismatched = first["NormalSlots"] + first["UnmatchedSlots"]
        this_null = first["NullSlots"] # not used
        this_total = first["TotalSlots"]

        for di in server_list:
            avg_num_of_rounds.append(di["avgNumOfRounds"])
            p95_num_of_rounds.append(di["p95NumOfrounds"])
            p99_num_of_rounds.append(di["p95NumOfrounds"])
            max_num_of_rounds.append(di["maxNumOfrounds"])
            round_dist_arrays.append(di["roundsDistribution"])

        con_ins_num_of_nor_mis_slots.append(this_normal_and_mismatched)
        con_ins_num_of_total_slots.append(this_total)

    # process the round distribution
    dist = defaultdict(lambda: 0)
    for array in round_dist_arrays:
        for i, e in enumerate(array):
            dist[i] += e

    trial["round distribution"] = dict(dist)

    trial["TotalSlots"] = {"p": "total # of slots (all instances)",
                           "v": sum(con_ins_num_of_total_slots)}
    trial["OkSlots"] = {"p": "total # of OK slots (all instances)",
                        "v": sum(con_ins_num_of_nor_mis_slots)}
    trial["AvgRound"] = {"p": "the average # of rounds / slot (all instances)",
                         "v": round(sum(avg_num_of_rounds) / len(avg_num_of_rounds), 2)}
    trial["P95Round"] = {"p": "the 95%tile # of rounds / slot (all instances)",
                         "v": round(sum(p95_num_of_rounds) / len(p95_num_of_rounds), 2)}
    trial["P99Round"] = {"p": "the 99%tile # of rounds / slot (all instances)",
                         "v": round(sum(p99_num_of_rounds) / len(p99_num_of_rounds), 2)}
    trial["MaxRound"] = {"p": "the max. # of rounds / slot (all instances)",
                         "v": max(max_num_of_rounds)}


'''
     Mar.28 2021: this is how we calculate the system throughput, using server-*-1.log
'''
def load_server_info(log_folder, param, trial):
    with concurrent.futures.ThreadPoolExecutor() as executor:
        futures = []
        for i in range(trial["ns"]["v"]):
            server_log_file_name = f"{param}--server-{i}-1.log"
            server_log_file_path = os.path.join(log_folder, server_log_file_name)
            futures.append(executor.submit(load_line_file, server_log_file_path))
        futures_res = [f.result() for f in futures]
        throughputs = []
        for throughput_logs in futures_res:  # for each machine
            machine_logs = []
            for interval_log in throughput_logs:  # for each interval
                interval_log = loads(interval_log)
                machine_logs.append(interval_log)
            machine_throughputs = [d["Interval throughput (cmd/sec)"] for d in machine_logs]
            # print(machine_throughputs)
            for idx, val in enumerate(machine_throughputs):  # remove head 0s
                if val != 0:
                    machine_throughputs = machine_throughputs[idx:]
                    break
            for idx, val in enumerate(machine_throughputs):  # remove tail 0s
                if val == 0:
                    machine_throughputs = machine_throughputs[:idx]
                    break
            # print(machine_throughputs)  # throughput vs. time
            # assert len(machine_throughputs) >= 10, f"the length is = {len(machine_throughputs)} (not enough runtime)"
            machine_mid80_throughputs = machine_throughputs[int(len(machine_throughputs) * 0.1):
                                                            int(len(machine_throughputs) * 0.9)]
            machine_mid80_throughput = sum(machine_mid80_throughputs) / len(machine_mid80_throughputs)
            throughputs.append(machine_mid80_throughput)
        throughput = sum(throughputs) / len(throughputs)
        trial["throughput"] = {"p": "throughput (ops/sec) -- server 80%", "v": round(throughput, 2)}


def print_statistics():
    if len(argv) == 1:
        log_folder = "../logs"
    else:
        log_folder = argv[1]
    trial_list = []
    for param in get_experiments(log_folder):
        sp = param.split("-")
        trial = {
            "ns": {"p": "Num of Servers", "v": int(sp[0][2:])},
            "nf": {"p": "Num of Faulty Servers", "v": int(sp[1][2:])},
            "nc": {"p": "Num of Clients", "v": int(sp[2][2:])},
            "nC": {"p": "Concurrency", "v": int(sp[3][2:])},
            "cr": {"p": "Client Timeout (s)", "v": int(sp[4][2:])},
            "ct": {"p": "Client Think Time (ms)", "v": int(sp[5][2:])},
            "cb": {"p": "Client Batch Size", "v": int(sp[6][2:])},
            "pb": {"p": "Proxy Batch Size", "v": int(sp[7][2:])},
            "pt": {"p": "Proxy Batch Timeout (ms)", "v": int(sp[8][2:])},
            "nb": {"p": "Network Batch Size - not used", "v": int(sp[9][2:])},
            "no": {"p": "Network Batch Timeout (ms) - not used", "v": int(sp[10][2:])},
            "client_log_dict_list": [],
            "client_send_time_list": [],
            "client_recv_time_list": [],
            "mid80RecvTimeDurs": [],
        }
        load_client_info(log_folder, param, trial)
        load_proxy_info(log_folder, param, trial)
        load_consensus_info(log_folder, param, trial)
        load_server_info(log_folder, param, trial)
        # print(trial)
        trial_list.append(trial)
    if len(trial_list) == 0:
        return
    if "print-title" in argv:
        print(trial_list[0]["throughput"]["p"], end=", ")
        for k, di in trial_list[0].items():
            if "p" in di:  # reportable parameter
                print(di["p"], end=", ")
        print()
    for trial in trial_list:
        print(trial["throughput"]["v"], end=", ")
        for k, di in trial.items():
            if "p" in di:  # reportable parameter
                print(di["v"], end=", ")
        print()
        if "print-round-dist" in argv:
            print("round distribution,", trial["round distribution"])
        print()


if __name__ == '__main__':
    print_statistics()
