def find_numeric(split_line):
    """Given a list of strings, return the first value that is numeric as an integer"""
    for section in split_line:
        if section.strip().isdigit():
            return int(section)
    return None


# Read client configuration to determine total number of requests sent.
with open('runClient.sh') as log:
    line = log.readline()
    while line != '':  # The EOF char is an empty string
        split_line = line.split("=")
        if len(split_line) == 2:
            if split_line[0] == 'NClient':
                n_clients = int(split_line[1])
            elif split_line[0] == 'NReq':
                n_req = int(split_line[1])
                break

        line = log.readline()

total_requests = n_clients * n_req

start_time_found = False

# Read client output to determine runtime.
with open('run.txt') as log:
    line = log.readline()
    while line != '':  # The EOF char is an empty string
        split_line = line.split(" ")
        time = find_numeric(split_line)

        if time is not None:
            if start_time_found:
                end_time = time
            else:
                start_time = time
                start_time_found = True

        line = log.readline()

runtime_in_nano_seconds = end_time - start_time
runtime_in_seconds = runtime_in_nano_seconds / pow(10, 9)

# Divide total number of request by runtime.
# print("runtime (seconds):", runtime_in_seconds)
# print("NClients:", n_clients)
# print("NReq:", n_req)
# print("total_requests:", total_requests)
print("Throughput (client requests per second):", total_requests / runtime_in_seconds)
