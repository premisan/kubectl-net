# kubectl-net

[English](README.md) | [한국어](README_ko.md)

**`kubectl-net`** 은 Kubernetes 클러스터 내부의 네트워크 및 DB 접속 문제를 실시간으로 진단하고 분석할 수 있는 종합 CLI 툴킷(kubectl 플러그인)입니다.

Kubernetes 표준 **Ephemeral Debug Container** 방식을 사용하여 GKE(Container-Optimized OS)나 온프레미스 `containerd` 런타임 버전 및 Distroless/Scratch 이미지와 관계없이 **1) 실시간 패킷 캡처(`cap`)**, **2) 파드 간/외부 curl 호출 테스트(`curl`)**, **3) ICMP ping 테스트(`ping`)**, **4) DNS 질의(`dig`)**, **5) Netcat 포트 접속/수신 테스트(`nc`)**, **6) 대화형 디버그 쉘(`sh`)**, **7) 특정 커맨드 주기적 반복 실행(`while`)**, **8) MySQL 접속/쿼리 테스트(`mysql`)**, **9) PostgreSQL 접속/쿼리 테스트(`psql`)**, **10) Redis 접속/명령 실행(`redis`)** 을 지원합니다.

---

## 💡 주요 특징 (Key Features)

* **GKE & 온프레미스 containerd 완벽 지원**:
  * 기존 `ksniff`와 달리 노드의 docker/containerd 런타임 소켓을 마운트하지 않으므로, 런타임 버전(1.6+, 2.0+) 및 노드 OS에 종속되지 않습니다.
* **보안 및 Distroless/Scratch 컨테이너 지원**:
  * 원본 컨테이너에 파일 복사나 재시작 없이 파드의 네트워크 네임스페이스에 Ephemeral 디버그 컨테이너를 부착합니다.
  * DB 클라이언트 서브커맨드(`mysql`, `psql`, `redis`)는 공식 `latest` 이미지(`mysql:latest`, `postgres:latest`, `redis:latest`)를 **비특권(non-privileged)** 모드로 부착하며, 이미지가 일치하는 실행 중 컨테이너만 안전하게 재사용합니다.
* **통일된 파라미터 위치 규칙 및 플래그 충돌 방지**:
  * `kubectl-net` 자체 플래그는 항상 **`POD_NAME` 앞**에 지정하고, **`POD_NAME` 뒤**에는 무조건 해당 커맨드(`curl`, `mysql`, `psql`, `redis-cli`, `ping`, `dig`, `nc`, `cap`, `while` 실행 커맨드)의 옵션과 인자만 지정합니다. 이를 통해 `-h`, `-n`, `-c`, `-p`, `-v` 등의 플래그 충돌을 원천적으로 방지합니다.

---

## 🚀 빠른 시작 (Quick Start)

### 0. 사전 설치 (macOS 기준)

`kubectl-net` 빌드 및 실시간 Wireshark 패킷 캡처 연동을 위해 [Homebrew](https://brew.sh)를 통해 아래 도구들을 사전에 설치해 주세요:

```bash
# 1. Go 언어 (빌드용) & kubectl (Kubernetes CLI)
brew install go kubectl

# 2. Wireshark (패킷 캡처 실시간 GUI 분석용)
brew install --cask wireshark

# 3. (선택 사항) tshark (터미널 stdout 파이프라인 패킷 분석용)
brew install tshark
```

---

### 1. 빌드 (Build)
```bash
make build
# 생성된 바이너리: ./bin/kubectl-net (및 심볼릭 링크 ./bin/kubectl_complete-net)
```

### 2. 설치 (Install as kubectl plugin)
기본 설치 경로는 사용자의 홈 디렉터리(`~/.local/bin`)입니다. `sudo` 권한 없이 안전하게 설치할 수 있습니다:

```bash
make install
# ~/.local/bin/kubectl-net 및 자동완성 헬퍼 ~/.local/bin/kubectl_complete-net 이 설치됩니다.
```

> [!NOTE]
> `~/.local/bin` 경로가 환경 변수 `PATH`에 등록되어 있어야 합니다. 등록되어 있지 않다면 셸 설정 파일(`~/.zshrc` 또는 `~/.bashrc`)에 아래 설정을 추가하세요:
> ```bash
> export PATH="$HOME/.local/bin:$PATH"
> ```

### 3. 셸 자동완성 설정 (Shell Completion)

`kubectl net` 커맨드는 Kubernetes 표준 플러그인 완성 규격(`kubectl_complete-*`)을 지원합니다.  
`make install` 시 `kubectl_complete-net` 헬퍼가 `~/.local/bin`에 함께 설치되므로, 셸에 `kubectl` 자체 자동완성이 설정되어 있다면 별도의 추가 설정 없이도 **`kubectl net cap -n <TAB>`**, **`kubectl net mysql -n prod <TAB>`** 등 네임스페이스/파드 자동완성이 즉시 작동합니다.

만약 `kubectl` 자동완성이 아직 설정되어 있지 않다면, 사용 중인 셸 설정 파일에 아래 라인을 추가하세요:

#### zsh (macOS 기본 셸)

`~/.zshrc` 파일에 추가:
```zsh
source <(kubectl completion zsh)
```
추가 후 `source ~/.zshrc` 를 실행합니다.

#### bash

`~/.bashrc` 파일에 추가:
```bash
source <(kubectl completion bash)
```
추가 후 `source ~/.bashrc` 를 실행합니다.

---

## 📐 통일된 CLI 파라미터 배치 규칙

모든 서브커맨드는 아래의 일관된 문법을 따릅니다:

```bash
kubectl net <서브커맨드> [kubectl-net 플래그...] <POD_NAME> [커맨드 옵션 및 인자...]
```

* **`<POD_NAME>` 앞**: `kubectl-net` 자체 플래그 (`-n/--namespace`, `-c/--container`, `--image`, `--kubeconfig`, `--context`, `-v/--verbose`, 그리고 `while`의 `-i/--interval`)
* **`<POD_NAME>` 뒤**: 실행 대상 커맨드 전용 옵션과 인자 (`curl`, `while`, `mysql`, `psql`, `redis`의 경우 원형 그대로 패스스루)

---

## 📖 서브커맨드 및 사용 예시 (Usage)

### 1. `kubectl net curl` (파드 네트워크 Curl 테스트)
특정 Pod의 네트워크 환경에서 다른 Pod, 내부 서비스 또는 외부 엔드포인트로 HTTP/HTTPS 요청을 보냅니다. **`POD_NAME` 뒤의 모든 인자는 기존 `curl` 플래그 그대로 전달**됩니다.

```bash
# 기본 GET 요청 (타겟 Pod -> 내부 서비스)
kubectl net curl my-pod http://backend-service.default.svc:8080/healthz

# 상세 출력(-v), SSL 인증서 무시(-k), 커스텀 헤더 및 POST 데이터 전송
kubectl net curl my-pod -v -k -X POST -H "Content-Type: application/json" \
  -d '{"name":"test"}' https://api.external.com/v1/data

# 네임스페이스/컨테이너 타겟팅 (kubectl-net 플래그는 POD_NAME 앞에 지정)
kubectl net curl -n production -c app-container my-pod --connect-timeout 3 -I https://google.com
```

### 2. `kubectl net ping` (ICMP 핑 테스트)
특정 Pod에서 다른 Pod IP, 노드 IP, 게이트웨이 또는 외부 IP로 ICMP Ping을 보내 네트워크 도달성과 지연시간(RTT)을 측정합니다.

```bash
# 다른 파드 IP로 핑 테스트 (기본 4회)
kubectl net ping my-pod 10.244.1.25

# 특정 네임스페이스 지정 및 핑 횟수(-C), 응답 대기시간(-t) 지정
kubectl net ping -n default my-pod 8.8.8.8 -C 10 -t 3
```

### 3. `kubectl net cap` (패킷 캡처 - 기존 kcap/ksniff 기능)
파드에서 발생하는 네트워크 패킷을 실시간으로 캡처하여 로컬 **Wireshark GUI**, **.pcap 파일**, 또는 **stdout (tshark 연동)** 으로 스트리밍합니다.

```bash
# Pod 트래픽 실시간 Wireshark GUI 실행 (기본)
kubectl net cap -n default my-pod

# HTTP(80) 및 HTTPS(443) 트래픽만 필터링 (BPF 필터)
kubectl net cap -n default my-pod -f "tcp port 80 or tcp port 443"

# 로컬 pcap 파일로 덤프 저장 (Wireshark GUI 없이 파일 저장)
kubectl net cap my-pod -o ./capture.pcap

# stdout으로 raw pcap 출력 후 tshark 연동
kubectl net cap my-pod -o - | tshark -r -
```

### 4. `kubectl net dig` (DNS 질의 진단)
파드 내부의 DNS 설정(CoreDNS)을 통해 내부 서비스 도메인 또는 외부 도메인에 대한 해석(Resolution) 상태를 확인합니다.

```bash
# 내부 서비스 도메인 질의
kubectl net dig my-pod backend-service.default.svc.cluster.local

# 외부 도메인 및 특정 레코드 타입 (SRV, TXT, AAAA 등) 질의
kubectl net dig -n default my-pod google.com -t AAAA

# 특정 DNS 서버 IP를 직접 지정하여 질의
kubectl net dig -n production my-pod kubernetes.default.svc.cluster.local -s 10.96.0.10
```

### 5. `kubectl net nc` (Netcat 포트 접속 및 수신 테스트)
특정 Pod에서 대상 호스트/IP의 포트로 TCP/UDP 연결 및 포트 오픈 상태를 점검하거나, 반대로 Pod 내에서 포트를 Listen(수신 대기)하여 다른 Pod로부터의 인바운드 연결 테스트를 수행합니다.

```bash
# 대상 서비스 포트 오픈 여부 빠른 점검 (Zero-I/O 포트 스캔)
kubectl net nc my-pod backend-service 8080 -z

# 특정 네임스페이스에서 IP 주소로 포트 접속 테스트 (연결 타임아웃 3초 지정)
kubectl net nc -n production my-pod 10.244.1.25 3306 -z -w 3

# DNS UDP 포트(53) 연결 테스트
kubectl net nc my-pod 10.96.0.10 53 -u -z

# 양방향 데이터 송수신 대화형 TCP 연결
kubectl net nc my-pod backend-service 8080

# 특정 포트로 수신 대기 (다른 Pod에서 이 Pod로 접속 테스트 가능)
kubectl net nc -n default my-pod -l 8080

# 연결이 끊겨도 계속 수신 대기 유지 (-k)
kubectl net nc my-pod -l 8080 -k

# UDP 5353 포트 수신 대기
kubectl net nc my-pod -l 5353 -u
```

### 6. `kubectl net sh` (대화형 디버그 쉘)
파드의 네트워크 네임스페이스를 공유하는 대화형 `netshoot` 쉘을 즉시 실행하여 터미널에서 자유롭게 네트워크 도구(`nmap`, `iperf3`, `netstat`, `traceroute` 등)를 실행합니다.

```bash
kubectl net sh -n default my-pod
```

### 7. `kubectl net while` (주기적 커맨드 반복 실행)
디버그 컨테이너 내부에서 `while true; do <command>; echo; sleep <interval>; done` 루프를 실행하여 특정 커맨드를 일정 간격으로 무한 반복합니다. `--interval`(`-i`)의 기본값은 `1`(1초)이며, 다른 `kubectl-net` 플래그와 함께 **`POD_NAME` 앞에 지정**합니다. `POD_NAME` 뒤에 이어지는 모든 파라미터는 디버그 컨테이너로 그대로 전달됩니다.

```bash
# 기본 간격(1초)으로 curl 반복 실행
kubectl net while my-pod curl -v https://ifconfig.me/

# 5초 간격으로 curl 반복 실행 (--interval 또는 -i 는 POD_NAME 앞에 지정)
kubectl net while --interval 5 my-pod curl -v https://ifconfig.me/
kubectl net while -i 5 my-pod curl -v https://ifconfig.me/

# 특정 네임스페이스 지정 및 2초 간격 반복 실행
kubectl net while -n production -i 2 my-pod curl -s -I http://backend-service:8080/healthz
```

### 8. `kubectl net mysql` (MySQL 접속 테스트 및 쿼리 실행)
타겟 Pod의 네트워크 네임스페이스에서 `mysql` 클라이언트로 MySQL/MariaDB 서버에 접속 테스트 및 쿼리를 수행합니다. 기본 디버그 이미지는 `mysql:latest`이며, `--image`로 오버라이드할 수 있습니다. 터미널 환경에서는 자동으로 TTY를 할당해 `-p` 비밀번호 프롬프트와 대화형 REPL을 안전하게 지원하며, 파이프/리다이렉션 시 자동으로 비대화형(non-TTY) 모드로 전환됩니다.

```bash
# 대화형 MySQL 접속 (비밀번호 프롬프트 입력)
kubectl net mysql my-pod -h mysql-svc.default.svc -u admin -p mydb

# 특정 네임스페이스 및 커스텀 포트 지정
kubectl net mysql -n production my-pod -h 10.0.10.25 -P 3306 -u appuser -p mydb

# 단일 테스트 쿼리 실행
kubectl net mysql -n production my-pod -h mysql-svc -u admin -p -e "SELECT 1;"

# 디버그 이미지 오버라이드 (예: MariaDB 또는 사설 레지스트리)
kubectl net mysql --image mariadb:latest -n production my-pod -h db.internal -u root -p
```

### 9. `kubectl net psql` (PostgreSQL 접속 테스트 및 쿼리 실행)
타겟 Pod의 네트워크 네임스페이스에서 `psql` 클라이언트로 PostgreSQL 서버에 접속 테스트 및 쿼리를 수행합니다. 기본 디버그 이미지는 `postgres:latest`이며, `--image`로 오버라이드할 수 있습니다.

```bash
# 대화형 PostgreSQL 접속
kubectl net psql my-pod -h postgres-svc.default.svc -U postgres -d mydb

# 특정 네임스페이스 지정 및 단일 쿼리 실행 (POD_NAME 뒤의 -c 는 psql 옵션으로 전달)
kubectl net psql -n production my-pod -h pg.internal -p 5432 -U appuser -d appdb -c "SELECT version();"

# PostgreSQL 접속 URI 사용
kubectl net psql -n production my-pod "postgresql://postgres@pg.internal:5432/mydb?sslmode=require"

# 디버그 이미지 오버라이드 (예: 경량 alpine 또는 사설 레지스트리)
kubectl net psql --image postgres:alpine my-pod -h pg-svc -U postgres
```

### 10. `kubectl net redis` (Redis 접속 테스트 및 명령 실행)
타겟 Pod의 네트워크 네임스페이스에서 `redis-cli`로 Redis 서버에 접속 테스트 및 명령을 수행합니다. 기본 디버그 이미지는 `redis:latest`이며, `--image`로 오버라이드할 수 있습니다.

```bash
# 대화형 redis-cli 세션 접속
kubectl net redis my-pod -h redis-svc.default.svc -p 6379

# 특정 네임스페이스에서 빠른 연결 확인 (PING)
kubectl net redis -n production my-pod -h redis-svc -p 6379 PING

# TLS Redis Cluster 접속 및 DB 인덱스 선택 (POD_NAME 뒤의 -c, -n 은 redis-cli 옵션으로 전달)
kubectl net redis -n production my-pod -h elasticache.internal -p 6379 --tls -c -n 0

# 디버그 이미지 오버라이드 (예: 경량 alpine 또는 사설 레지스트리)
kubectl net redis --image redis:alpine my-pod -h redis-svc PING
```

---

## ⚙️ 공통 플래그 (Global Flags — 항상 `POD_NAME` 앞에 지정)

| 플래그 | 단축키 | 기본값 | 설명 |
| :--- | :--- | :--- | :--- |
| `--namespace` | `-n` | 현재 context 네임스페이스 | Kubernetes 네임스페이스 |
| `--container` | `-c` | 첫 번째 컨테이너 | 타겟 컨테이너 이름 |
| `--image` | | `nicolaka/netshoot:latest`<br>(DB 서브커맨드: `mysql:latest` / `postgres:latest` / `redis:latest`) | Ephemeral 디버그 컨테이너 이미지 오버라이드 |
| `--kubeconfig` | | `~/.kube/config` | Kubeconfig 파일 경로 |
| `--context` | | 현재 context | Kubeconfig Context 이름 |
| `--verbose` | `-v` | `false` | 상세 디버그 로그 활성화 |

---

## 🛠️ 지원 환경
- **Kubernetes**: GKE, EKS, AKS, 온프레미스 (k3s, kubeadm, RKE2, Talos 등) v1.23+
- **컨테이너 런타임**: containerd (모든 버전), CRI-O, Docker
- **운영체제**: macOS, Linux, Windows

---

## 🗑️ 플러그인 삭제 (Uninstallation)

설치된 `kubectl-net` 플러그인 및 자동완성 헬퍼를 삭제하려면 아래 명령어를 실행하세요:

```bash
make uninstall
# 또는 수동 삭제:
# rm -f ~/.local/bin/kubectl-net ~/.local/bin/kubectl_complete-net
```
