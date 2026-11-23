# kubectl-net

[English](README.md) | [한국어](README_ko.md)

**`kubectl-net`** is a comprehensive CLI toolkit and `kubectl` plugin designed for real-time network and database connectivity troubleshooting inside Kubernetes clusters.

Leveraging the standard Kubernetes **Ephemeral Debug Containers** mechanism, it operates seamlessly across GKE (Container-Optimized OS) and on-premise `containerd` environments, and works effortlessly with Distroless or Scratch images. It provides:

1. **Real-time Packet Capture (`cap`)**
2. **Pod-to-Pod & External HTTP(S) Request Testing (`curl`)**
3. **ICMP Ping Testing (`ping`)**
4. **DNS Resolution Diagnostics (`dig`)**
5. **Netcat Port Connectivity & Listen Testing (`nc`)**
6. **Telnet TCP Connectivity & Banner Testing (`telnet`)**
7. **Interactive Network Debug Shell (`sh`)**
8. **Repeated Loop Command Execution (`while`)**
9. **MySQL Client Connectivity & Querying (`mysql`)**
10. **PostgreSQL Client Connectivity & Querying (`psql`)**
11. **Redis CLI Connectivity & Command Execution (`redis`)**

---

## 💡 Key Features

* **Full Support for GKE & On-premise containerd**:
  * Unlike legacy tools such as `ksniff`, it does not mount the node's docker or containerd runtime socket. As a result, it is completely independent of runtime versions (containerd 1.6+, 2.0+) and host node OS distributions.
* **Security & Distroless / Scratch Container Support**:
  * Attaches an ephemeral debug container directly into the target pod's network namespace without restarting the pod or copying files into the original container.
  * DB client subcommands (`mysql`, `psql`, `redis`) attach **non-privileged** ephemeral containers using official `latest` images (`mysql:latest`, `postgres:latest`, `redis:latest`) and reuse existing running containers only when the image matches.
* **Unified Argument Position & Conflict-Free Flag Passthrough**:
  * All `kubectl-net` flags are placed **before `POD_NAME`**, and everything **after `POD_NAME`** belongs exclusively to the target command (`curl`, `telnet`, `mysql`, `psql`, `redis-cli`, `ping`, `dig`, `nc`, `cap`, `while`). This completely avoids flag collisions such as `-h`, `-n`, `-c`, `-p`, and `-v`.

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
# Generated binaries: ./bin/kubectl-net (and helper ./bin/kubectl_complete-net)
```

### 2. Install (as kubectl plugin)
The default install directory is `~/.local/bin` in your user home directory, requiring no `sudo` privileges:

```bash
make install
# Installs ~/.local/bin/kubectl-net and completion helper ~/.local/bin/kubectl_complete-net
```

> [!NOTE]
> Make sure `~/.local/bin` is in your `PATH`. If not, add the following to your shell configuration (`~/.zshrc` or `~/.bashrc`):
> ```bash
> export PATH="$HOME/.local/bin:$PATH"
> ```

### 3. Shell Completion

`kubectl-net` adheres to Kubernetes' native plugin completion specification (`kubectl_complete-*`).  
When running `make install`, the `kubectl_complete-net` helper is automatically installed to `~/.local/bin`. As long as standard `kubectl` completion is enabled in your shell, **dynamic tab completion for pods, namespaces, and flags will work out-of-the-box** (e.g., `kubectl net cap -n <TAB>`, `kubectl net mysql -n prod <TAB>`) without needing extra `compdef` wrappers.

If you haven't enabled `kubectl` shell completion yet, add the following to your shell configuration:

#### zsh (macOS Default Shell)

Add to `~/.zshrc`:
```zsh
source <(kubectl completion zsh)
```
Then run `source ~/.zshrc`.

#### bash

Add to `~/.bashrc`:
```bash
source <(kubectl completion bash)
```
Then run `source ~/.bashrc`.

---

## 📐 Unified CLI Argument Convention

All subcommands follow a single consistent syntax:

```bash
kubectl net <subcommand> [KUBECTL_NET_FLAGS...] <POD_NAME> [COMMAND_OPTIONS_AND_ARGS...]
```

* **Before `<POD_NAME>`**: `kubectl-net` flags (`-n/--namespace`, `-c/--container`, `--image`, `--kubeconfig`, `--context`, `-v/--verbose`, and `-i/--interval` for `while`).
* **After `<POD_NAME>`**: Options and arguments belonging solely to the target command (passed verbatim for `curl`, `telnet`, `while`, `mysql`, `psql`, and `redis`).

---

## 📖 Subcommands & Usage

### 1. `kubectl net curl` (Pod Network Curl Test)
Send HTTP/HTTPS requests directly from inside a target pod's network environment to other pods, internal cluster services, or external endpoints. **All standard curl flags after `POD_NAME` are passed verbatim**.

```bash
# Basic GET request (Target Pod -> Internal Service)
kubectl net curl my-pod http://backend-service.default.svc:8080/healthz

# Verbose output (-v), insecure SSL (-k), custom headers, and POST data payload
kubectl net curl my-pod -v -k -X POST -H "Content-Type: application/json" \
  -d '{"name":"test"}' https://api.external.com/v1/data

# Target a specific namespace and container (kubectl-net flags before POD_NAME)
kubectl net curl -n production -c app-container my-pod --connect-timeout 3 -I https://google.com
```

### 2. `kubectl net ping` (ICMP Ping Test)
Send ICMP Echo requests from inside a target pod to another pod IP, node IP, gateway, or external IP to verify reachability and measure round-trip time (RTT).

```bash
# Ping another pod IP (default: 4 packets)
kubectl net ping my-pod 10.244.1.25

# Target pod in a specific namespace with custom packet count (-C) and timeout (-t)
kubectl net ping -n default my-pod 8.8.8.8 -C 10 -t 3
```

### 3. `kubectl net cap` (Packet Capture)
Capture network packets on a target pod in real time and stream them directly into a local **Wireshark GUI**, export to a **.pcap file**, or pipe to **stdout (tshark integration)**.

```bash
# Stream pod traffic directly to Wireshark GUI in real time (default)
kubectl net cap -n default my-pod

# Filter traffic using BPF expressions (e.g., HTTP 80 and HTTPS 443)
kubectl net cap -n default my-pod -f "tcp port 80 or tcp port 443"

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
kubectl net dig -n default my-pod google.com -t AAAA

# Query directly via a specific DNS server IP
kubectl net dig -n production my-pod kubernetes.default.svc.cluster.local -s 10.96.0.10
```

### 5. `kubectl net nc` (Netcat Port Connectivity & Listen Test)
Inspect TCP/UDP port connectivity and open ports towards a destination host/IP, or listen on a specific port inside the pod to test inbound connectivity from other pods.

```bash
# Quick port scan / zero-I/O check on a service port
kubectl net nc my-pod backend-service 8080 -z

# Test port connectivity from a pod in a specific namespace with a 3-second timeout
kubectl net nc -n production my-pod 10.244.1.25 3306 -z -w 3

# Test UDP port connectivity (e.g., DNS port 53)
kubectl net nc my-pod 10.96.0.10 53 -u -z

# Interactive two-way TCP communication
kubectl net nc my-pod backend-service 8080

# Listen on a specific port (test incoming connections from other pods)
kubectl net nc -n default my-pod -l 8080

# Keep listening across multiple connections (-k)
kubectl net nc my-pod -l 8080 -k

# Listen on a UDP port
kubectl net nc my-pod -l 5353 -u
```

### 6. `kubectl net telnet` (Telnet TCP Connectivity Test)
Test TCP port connectivity and inspect protocol banners (HTTP, SMTP, Redis, etc.) interactively from inside a target pod using `telnet` (bundled in `nicolaka/netshoot:latest` via `busybox-extras`). All arguments after `POD_NAME` are passed verbatim to `telnet`.

```bash
# Test TCP connectivity to an internal service port
kubectl net telnet my-pod backend-service 8080

# Target pod in a specific namespace
kubectl net telnet -n production my-pod 10.244.1.25 3306

# Inspect service banner interactively (press Ctrl+] then 'quit' to exit)
kubectl net telnet -n default my-pod mail.internal 25
```

### 7. `kubectl net sh` (Interactive Debug Shell)
Launch an interactive `netshoot` shell sharing the target pod's network namespace, granting full access to diagnostic utilities such as `nmap`, `iperf3`, `netstat`, `traceroute`, `tcpdump`, and more.

```bash
kubectl net sh -n default my-pod
```

### 8. `kubectl net while` (Repeated Loop Command Execution)
Repeatedly execute any command at a fixed interval inside the target pod's debug container (`while true; do <command>; echo; sleep <interval>; done`). `--interval` (`-i`) defaults to `1` second and must be placed **before `POD_NAME`** alongside any other `kubectl-net` flags. Everything after `POD_NAME` is passed verbatim to the debug container.

```bash
# Repeat curl every 1 second (default interval)
kubectl net while my-pod curl -v https://ifconfig.me/

# Repeat curl every 5 seconds (--interval or -i before POD_NAME)
kubectl net while --interval 5 my-pod curl -v https://ifconfig.me/
kubectl net while -i 5 my-pod curl -v https://ifconfig.me/

# Target a specific namespace and repeat every 2 seconds
kubectl net while -n production -i 2 my-pod curl -s -I http://backend-service:8080/healthz
```

### 9. `kubectl net mysql` (MySQL Client Test & Query)
Connect to a MySQL/MariaDB server from inside a target pod's network namespace using an Ephemeral Debug Container (default image: `mysql:latest`, override via `--image`). Interactive sessions automatically allocate a TTY for safe password prompts (`-p`) and REPL queries; piped or redirected I/O automatically switches to non-TTY mode.

```bash
# Interactive MySQL connection with password prompt
kubectl net mysql my-pod -h mysql-svc.default.svc -u admin -p mydb

# Target pod in a specific namespace and custom port
kubectl net mysql -n production my-pod -h 10.0.10.25 -P 3306 -u appuser -p mydb

# Execute a single test query
kubectl net mysql -n production my-pod -h mysql-svc -u admin -p -e "SELECT 1;"

# Override debug container image (e.g., MariaDB or private registry)
kubectl net mysql --image mariadb:latest -n production my-pod -h db.internal -u root -p
```

### 10. `kubectl net psql` (PostgreSQL Client Test & Query)
Connect to a PostgreSQL server from inside a target pod's network namespace using `psql` (default image: `postgres:latest`, override via `--image`).

```bash
# Interactive PostgreSQL session
kubectl net psql my-pod -h postgres-svc.default.svc -U postgres -d mydb

# Target pod in a specific namespace and run a single query (-c belongs to psql after POD_NAME)
kubectl net psql -n production my-pod -h pg.internal -p 5432 -U appuser -d appdb -c "SELECT version();"

# Connect via PostgreSQL URI
kubectl net psql -n production my-pod "postgresql://postgres@pg.internal:5432/mydb?sslmode=require"

# Override debug container image (e.g., lightweight alpine or private registry)
kubectl net psql --image postgres:alpine my-pod -h pg-svc -U postgres
```

### 11. `kubectl net redis` (Redis CLI Test & Command Execution)
Connect to a Redis server from inside a target pod's network namespace using `redis-cli` (default image: `redis:latest`, override via `--image`).

```bash
# Interactive redis-cli session
kubectl net redis my-pod -h redis-svc.default.svc -p 6379

# Quick connectivity check (PING) from a pod in a specific namespace
kubectl net redis -n production my-pod -h redis-svc -p 6379 PING

# TLS-enabled Redis Cluster with DB selection (-c and -n after POD_NAME belong to redis-cli)
kubectl net redis -n production my-pod -h elasticache.internal -p 6379 --tls -c -n 0

# Override debug container image (e.g., lightweight alpine or private registry)
kubectl net redis --image redis:alpine my-pod -h redis-svc PING
```

---

## ⚙️ Global Flags (Place Before `POD_NAME`)

| Flag | Shorthand | Default | Description |
| :--- | :--- | :--- | :--- |
| `--namespace` | `-n` | Current context namespace | Kubernetes namespace |
| `--container` | `-c` | First container | Target container name |
| `--image` | | `nicolaka/netshoot:latest`<br>(`mysql:latest` / `postgres:latest` / `redis:latest` for DB subcommands) | Ephemeral debug container image override |
| `--kubeconfig` | | `~/.kube/config` | Path to kubeconfig file |
| `--context` | | Current context | Name of kubeconfig context |
| `--verbose` | `-v` | `false` | Enable verbose debug logging |

---

## 🛠️ Supported Environments
- **Kubernetes**: GKE, EKS, AKS, On-premise (k3s, kubeadm, RKE2, Talos, etc.) v1.23+
- **Container Runtimes**: containerd (all versions), CRI-O, Docker
- **Operating Systems**: macOS, Linux, Windows

---

## 🗑️ Uninstallation

To remove the `kubectl-net` plugin and its completion helper:

```bash
make uninstall
# Or manually remove:
# rm -f ~/.local/bin/kubectl-net ~/.local/bin/kubectl_complete-net
```
