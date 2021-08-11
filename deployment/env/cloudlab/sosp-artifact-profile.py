"""SOSP 2021: Rabia Artifact Testing Profile RawPC

m510:
Type: (Intel Xeon-D)
CPU: Eight-core Intel Xeon D-1548 at 2.0 GHz
RAM: 64GB ECC Memory (4x 16 GB DDR4-2133 SO-DIMMs)
Disk: 256 GB NVMe flash storage
NIC: Dual-port Mellanox ConnectX-3 10 GB NIC (PCIe v3.0, 8 lanes

d6515:
Type: (AMD EPYC Rome, 32 core, 2 disk, 100Gb Ethernet)
CPU: 32-core AMD 7452 at 2.35GHz
RAM: 128GB ECC Memory (8x 16 GB 3200MT/s RDIMMs)
Disk: Two 480 GB 6G SATA SSD
NIC: Dual-port Mellanox ConnectX-5 100 GB NIC (PCIe v4.0)
NIC: Dual-port Broadcom 57414 25 GB NIC

Instructions:
1. See docs/run-rabia.md and video for full setup instructions.
"""
import geni.portal as portal

request = portal.context.makeRequestRSpec()


DEFAULT_SVR_HARDWARE_TYPE = "m510"
DEFAULT_CLI_HARDWARE_TYPE = "d6515"
DEFAULT_DISK_IMAGE = "urn:publicid:IDN+emulab.net+image+emulab-ops:UBUNTU18-64-STD" # @todo: add custom disk image?
DEFAULT_LAN_SOCKET = "eth1"

lan = request.LAN()
svr_1 = request.RawPC("svr_1")
svr_2 = request.RawPC("svr_2")
svr_3 = request.RawPC("svr_3")
cli_1 = request.RawPC("cli_1")
cli_2 = request.RawPC("cli_2")
cli_3 = request.RawPC("cli_3")

svrs = [svr_1, svr_2, svr_3]
clis = [cli_1, cli_2, cli_3]

for svr, cli in zip(svrs, clis):
    svr.hardware_type, cli.hardware_type = DEFAULT_SVR_HARDWARE_TYPE, DEFAULT_CLI_HARDWARE_TYPE
    svr.disk_image, cli.disk_image = DEFAULT_DISK_IMAGE, DEFAULT_DISK_IMAGE
    s_iface, c_iface = svr.addInterface(DEFAULT_LAN_SOCKET), cli.addInterface(DEFAULT_LAN_SOCKET)
    lan.addInterface(s_iface)
    lan.addInterface(c_iface)


portal.context.printRequestRSpec(request)
