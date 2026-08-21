# kubectl-net (knet)

**`kubectl-net` (`knet`)** 은 Kubernetes 클러스터 내부의 네트워크 문제를 실시간으로 진단하고 분석할 수 있는 종합 CLI 툴킷(kubectl 플러그인)입니다.

Kubernetes 표준 **Ephemeral Debug Container** 방식을 사용하여 GKE(Container-Optimized OS)나 온프레미스 `containerd` 런타임 버전 및 Distroless/Scratch 이미지와 관계없이 **1) 실시간 패킷 캡처(`cap`)**, **2) 파드 간/외부 curl 호출 테스트(`curl`)**, **3) ICMP ping 테스트(`ping`)**, **4) DNS 질의(`dig`)**, **5) 대화형 디버그 쉘(`sh`)** 을 지원합니다.

---

## 💡 주요 특징 (Key Features)

* **GKE & 온프레미스 containerd 완벽 지원**:
  * 기존 `ksniff`와 달리 노드의 docker/containerd 런타임 소켓을 마운트하지 않으므로, 런타임 버전(1.6+, 2.0+) 및 노드 OS에 종속되지 않습니다.
* **보안 및 Distroless/Scratch 컨테이너 지원**:
  * 원본 컨테이너에 파일 복사나 재시작 없이 파드의 네트워크 네임스페이스에 디버그 컨테이너(`netshoot`)를 부착합니다.
* **기존 `curl` 플래그 전면 지원**:
  * `-v`, `-k`, `-X POST`, `-H`, `-d`, `--connect-timeout` 등 기존 curl 커맨드의 모든 옵션을 그대로 사용할 수 있습니다.

---

## 🚀 빠른 시작 (Quick Start)

### 1. 빌드 (Build)
```bash
make build
# 생성된 바이너리: ./bin/kubectl-net (및 alias ./bin/knet)
```

### 2. 설치 (Install as kubectl plugin)
```bash
make install
# 이제 'kubectl net <명령어>' 또는 'knet <명령어>'로 어디서든 실행할 수 있습니다.
```

---

## 📖 서브커맨드 및 사용 예시 (Usage)

### 1. `kubectl net curl` (파드 네트워크 Curl 테스트)
특정 Pod의 네트워크 환경에서 다른 Pod, 내부 서비스 또는 외부 엔드포인트로 HTTP/HTTPS 요청을 보냅니다. **기존 curl의 모든 플래그를 그대로 사용**할 수 있습니다.

```bash
# 기본 GET 요청 (타겟 Pod -> 내부 서비스)
kubectl net curl my-pod http://backend-service.default.svc:8080/healthz

# 상세 출력(-v), SSL 인증서 무시(-k), 커스텀 헤더 및 POST 데이터 전송
kubectl net curl my-pod -v -k -X POST -H "Content-Type: application/json" \
  -d '{"name":"test"}' https://api.external.com/v1/data

# 연결 타임아웃 지정 및 네임스페이스/컨테이너 타겟팅
kubectl net curl my-pod -n production -c app-container --connect-timeout 3 -I https://google.com
```

### 2. `kubectl net ping` (ICMP 핑 테스트)
특정 Pod에서 다른 Pod IP, 노드 IP, 게이트웨이 또는 외부 IP로 ICMP Ping을 보내 네트워크 도달성과 지연시간(RTT)을 측정합니다.

```bash
# 다른 파드 IP로 핑 테스트 (기본 4회)
kubectl net ping my-pod 10.244.1.25

# 핑 횟수(-C) 및 응답 대기시간(-t) 지정
kubectl net ping my-pod 8.8.8.8 -C 10 -t 3 -n default
```

### 3. `kubectl net cap` (패킷 캡처 - 기존 kcap/ksniff 기능)
파드에서 발생하는 네트워크 패킷을 실시간으로 캡처하여 로컬 **Wireshark GUI**, **.pcap 파일**, 또는 **stdout (tshark 연동)** 으로 스트리밍합니다.

```bash
# Pod 트래픽 실시간 Wireshark GUI 실행 (기본)
kubectl net cap my-pod -n default

# HTTP(80) 및 HTTPS(443) 트래픽만 필터링 (BPF 필터)
kubectl net cap my-pod -f "tcp port 80 or tcp port 443"

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
kubectl net dig my-pod google.com -t AAAA

# 특정 DNS 서버 IP를 직접 지정하여 질의
kubectl net dig my-pod kubernetes.default.svc.cluster.local -s 10.96.0.10
```

### 5. `kubectl net sh` (대화형 디버그 쉘)
파드의 네트워크 네임스페이스를 공유하는 대화형 `netshoot` 쉘을 즉시 실행하여 터미널에서 자유롭게 네트워크 도구(`nmap`, `iperf3`, `netstat`, `traceroute` 등)를 실행합니다.

```bash
kubectl net sh my-pod -n default
```

---

## ⚙️ 공통 플래그 (Global Flags)

| 플래그 | 단축키 | 기본값 | 설명 |
| :--- | :--- | :--- | :--- |
| `--namespace` | `-n` | 현재 context 네임스페이스 | Kubernetes 네임스페이스 |
| `--container` | `-c` | 첫 번째 컨테이너 | 타겟 컨테이너 이름 |
| `--image` | | `nicolaka/netshoot:latest` | Ephemeral 디버그 컨테이너 이미지 |
| `--kubeconfig` | | `~/.kube/config` | Kubeconfig 파일 경로 |
| `--context` | | 현재 context | Kubeconfig Context 이름 |
| `--verbose` | `-v` | `false` | 상세 디버그 로그 활성화 |

---

## 🛠️ 지원 환경
- **Kubernetes**: GKE, EKS, AKS, 온프레미스 (k3s, kubeadm, RKE2, Talos 등) v1.23+
- **컨테이너 런타임**: containerd (모든 버전), CRI-O, Docker
- **운영체제**: macOS, Linux, Windows
