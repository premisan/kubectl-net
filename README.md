# kubectl-net (knet)

[English](README.md) | [한국어](README_ko.md)

**`kubectl-net` (`knet`)** is a comprehensive CLI toolkit and `kubectl` plugin designed for real-time network troubleshooting and diagnostics inside Kubernetes clusters.

Leveraging the standard Kubernetes **Ephemeral Debug Containers** mechanism, it operates seamlessly across GKE (Container-Optimized OS) and on-premise `containerd` environments, and works effortlessly with Distroless or Scratch images. It provides:

1. **Real-time Packet Capture (`cap`)**
2. **Pod-to-Pod & External HTTP(S) Request Testing (`curl`)**
3. **ICMP Ping Testing (`ping`)**
4. **DNS Resolution Diagnostics (`dig`)**
5. **Netcat Port Connectivity & Listen Testing (`nc`)**
6. **Interactive Network Debug Shell (`sh`)**

---

## 💡 Key Features

* **Full Support for GKE & On-premise containerd**:
  * Unlike legacy tools such as `ksniff`, it does not mount the node's docker or containerd runtime socket. As a result, it is completely independent of runtime versions (containerd 1.6+, 2.0+) and host node OS distributions.
* **Security & Distroless / Scratch Container Support**:
  * Attaches a debug container (`netshoot`) directly into the target pod's network namespace without restarting the pod or copying files into the original container.
* **Full Support for Standard `curl` Flags**:
  * Pass standard curl options seamlessly, including `-v`, `-k`, `-X POST`, `-H`, `-d`, `--connect-timeout`, and more.

---

## 🚀 Quick Start

### 0. Prerequisites (macOS Example)

To build `kubectl-net` and utilize real-time Wireshark packet capture integration, install the following tools via [Homebrew](https://brew.sh):

```bash
# 1. Go (for building) & kubectl (Kubernetes CLI)
brew install go kubectl

# 2. Wireshark (for real-time GUI packet analysis)
brew install --cask wireshark

# 3. (Optional) tshark (for terminal stdout pipeline packet analysis)
brew install tshark
```

---

### 1. Build
```bash
make build
# Generated binaries: ./bin/kubectl-net (and alias ./bin/knet)
```

### 2. Install (as kubectl plugin)
```bash
make install
# Now executable anywhere via 'kubectl net <command>' or 'knet <command>'
```

### 3. Shell Completion

Configure tab completion for pod names and flags (e.g., `knet cap <TAB>`).

#### zsh (macOS Default Shell)

Add the following to your `~/.zshrc` and run `source ~/.zshrc`:

```zsh
# knet shell completion
source <(knet completion zsh)
compdef _kubectl-net knet
```

#### bash

Add the following to your `~/.bashrc`:

```bash
# knet shell completion
source <(knet completion bash)
compdef _kubectl-net knet
```

---

## 📖 Subcommands & Usage

### 1. `kubectl net curl` (Pod Network Curl Test)
Send HTTP/HTTPS requests directly from inside a target pod's network environment to other pods, internal cluster services, or external endpoints. **All standard curl flags are supported**.

```bash
# Basic GET request (Target Pod -> Internal Service)
kubectl net curl my-pod http://backend-service.default.svc:8080/healthz

# Verbose output (-v), insecure SSL (-k), custom headers, and POST data payload
kubectl net curl my-pod -v -k -X POST -H "Content-Type: application/json" \
  -d '{"name":"test"}' https://api.external.com/v1/data

# Connect timeout and namespace/container targeting
kubectl net curl my-pod -n production -c app-container --connect-timeout 3 -I https://google.com
```

### 2. `kubectl net ping` (ICMP Ping Test)
Send ICMP Echo requests from inside a target pod to another pod IP, node IP, gateway, or external IP to verify reachability and measure round-trip time (RTT).

```bash
# Ping another pod IP (default: 4 packets)
kubectl net ping my-pod 10.244.1.25

# Specify packet count (-C) and response timeout (-t)
kubectl net ping my-pod 8.8.8.8 -C 10 -t 3 -n default
```

### 3. `kubectl net cap` (Packet Capture)
Capture network packets on a target pod in real time and stream them directly into a local **Wireshark GUI**, export to a **.pcap file**, or pipe to **stdout (tshark integration)**.

```bash
# Stream pod traffic directly to Wireshark GUI in real time (default)
kubectl net cap my-pod -n default

# Filter traffic using BPF expressions (e.g., HTTP 80 and HTTPS 443)
kubectl net cap my-pod -f "tcp port 80 or tcp port 443"

# Dump capture directly to a local .pcap file (headless mode)
kubectl net cap my-pod -o ./capture.pcap

# Stream raw pcap to stdout and pipe into tshark
kubectl net cap my-pod -o - | tshark -r -
```

### 4. `kubectl net dig` (DNS Resolution Diagnostics)
Inspect DNS resolution via the pod's DNS configuration (CoreDNS) for cluster-internal service names or external domains.

```bash
# Query an internal service domain
kubectl net dig my-pod backend-service.default.svc.cluster.local

# Query external domains and specific record types (SRV, TXT, AAAA, etc.)
kubectl net dig my-pod google.com -t AAAA

# Query directly via a specific DNS server IP
kubectl net dig my-pod kubernetes.default.svc.cluster.local -s 10.96.0.10
```

### 5. `kubectl net nc` (Netcat Port Connectivity & Listen Test)
Inspect TCP/UDP port connectivity and open ports towards a destination host/IP, or listen on a specific port inside the pod to test inbound connectivity from other pods.

```bash
# Quick port scan / zero-I/O check on a service port
kubectl net nc my-pod backend-service 8080 -z

# Test port connectivity with a 3-second connection timeout
kubectl net nc my-pod 10.244.1.25 3306 -z -w 3

# Test UDP port connectivity (e.g., DNS port 53)
kubectl net nc my-pod 10.96.0.10 53 -u -z

# Interactive two-way TCP communication
kubectl net nc my-pod backend-service 8080

# Listen on a specific port (test incoming connections from other pods)
kubectl net nc my-pod -l 8080

# Keep listening across multiple connections (-k)
kubectl net nc my-pod -l 8080 -k

# Listen on a UDP port
kubectl net nc my-pod -l 5353 -u
```

### 6. `kubectl net sh` (Interactive Debug Shell)
Launch an interactive `netshoot` shell sharing the target pod's network namespace, granting full access to diagnostic utilities such as `nmap`, `iperf3`, `netstat`, `traceroute`, `tcpdump`, and more.

```bash
kubectl net sh my-pod -n default
```

---

## ⚙️ Global Flags

| Flag | Shorthand | Default | Description |
| :--- | :--- | :--- | :--- |
| `--namespace` | `-n` | Current context namespace | Kubernetes namespace |
| `--container` | `-c` | First container | Target container name |
| `--image` | | `nicolaka/netshoot:latest` | Ephemeral debug container image |
| `--kubeconfig` | | `~/.kube/config` | Path to kubeconfig file |
| `--context` | | Current context | Name of kubeconfig context |
| `--verbose` | `-v` | `false` | Enable verbose debug logging |

---

## 🛠️ Supported Environments
- **Kubernetes**: GKE, EKS, AKS, On-premise (k3s, kubeadm, RKE2, Talos, etc.) v1.23+
- **Container Runtimes**: containerd (all versions), CRI-O, Docker
- **Operating Systems**: macOS, Linux, Windows
