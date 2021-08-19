# NOTE: There is a rare error where log.out may contain a non utf-encoded character.
# Simply rerun the experiment if this happens.

import numpy as np

latencies = []

with open('log.out') as log:
    line = log.readline()
    while line != '':  # The EOF char is an empty string
        # Extract the latency and its unit of time.
        if len(line.split(" ")) == 3:
            latency_unparsed = line.split(" ")[2]
        # If the output is irregular, ignore it.
        else:
            line = log.readline()
            continue

        # Remove the unit of time and add the latency to a list,
        # converting the latency to milliseconds if it is in microseconds.
        if latency_unparsed.find('µs') != -1:
            latencies.append(float(latency_unparsed[0:(len(latency_unparsed) - 3)]) / 1000)
        elif latency_unparsed.find('ms') != -1:
            latencies.append(float(latency_unparsed[0:(len(latency_unparsed) - 3)]))
        else:
            print("Error, parsing code does not work")
            exit()

        line = log.readline()

print("Median and 99th percentile latency in milliseconds:")
print(str(np.percentile(latencies, 50)) + ", " + str(np.percentile(latencies, 99)))
