# kcap (Kubernetes Packet Capture CLI)

`kcap`은 Kubernetes 클러스터 내부의 Pod/컨테이너에서 발생하는 네트워크 패킷을 실시간으로 캡처하여 로컬 머신의 **Wireshark GUI**, **.pcap 파일**, 또는 **stdout (tshark 등 파이프라인)**으로 스트리밍하는 모던 CLI 도구입니다.

---

## 💡 왜 ksniff 대신 kcap인가요?

기존 `ksniff`는 다음과 같은 환경에서 캡처가 실패하는 한계가 있었습니다:

1. **containerd 런타임 종속성 문제 (`ksniff -p`)**:
   - 노드의 `/var/run/docker.sock` 또는 특정 containerd 소켓 경로에 하드코딩 의존하여, GKE(Container-Optimized OS)나 최신 온프레미스 containerd 버전(1.6+, 2.0+)에서 컨테이너를 찾지 못하고 실패합니다.
2. **보안 제약 (Distroless / ReadOnly Root Filesystem / Non-Root)**:
   - Pod 내부 `/tmp`에 static 바이너리를 업로드할 수 없어 실패합니다.

### kcap의 해결 방식: K8s 네이티브 Ephemeral Container
- Kubernetes v1.23+ 표준 **Ephemeral Containers API (`pods/ephemeralcontainers`)**를 기반으로 동작합니다.
- **노드 OS(GKE COS, Ubuntu, RHEL, Talos) 및 컨테이너 런타임(containerd, CRI-O, Docker 버전 무관)에 일체 영향을 받지 않습니다.**
- Distroless, Scratch, Non-root 컨테이너에서도 별도 파일 업로드 없이 즉시 패킷을 캡처할 수 있습니다.

---

## 🚀 빠른 시작 (Quick Start)

### 1. 빌드 (Build)
```bash
make build
# 빌드된 바이너리: ./bin/kcap
```

### 2. kubectl 플러그인으로 설치 (선택)
```bash
make install
# 이제 'kcap' 또는 'kubectl cap' 명령어로 바로 실행할 수 있습니다.
```

---

## 📖 사용 예시 (Usage Examples)

### 1. 실시간 Wireshark GUI 캡처 (기본)
Pod 이름을 지정하면 백그라운드에서 패킷 스트림을 수신하고 로컬 Wireshark GUI가 자동으로 실행됩니다.
```bash
# 기본 네임스페이스의 my-pod 캡처
kcap my-pod

# 특정 네임스페이스 및 컨테이너 지정
kcap my-pod -n production -c my-app
```

### 2. BPF 패킷 필터 적용
HTTP/HTTPS 트래픽 또는 특정 IP만 필터링하여 캡처합니다.
```bash
# HTTP(80) 및 HTTPS(443) 포트 트래픽만 캡처
kcap my-pod -n default -f "tcp port 80 or tcp port 443"

# 특정 호스트와의 통신만 캡처
kcap my-pod -f "host 10.0.0.15"
```

### 3. 로컬 .pcap 파일로 저장 (Wireshark GUI 없이 백그라운드 덤프)
```bash
kcap my-pod -n default -o ./traffic_dump.pcap -f "port 8080"
```

### 4. stdout 스트림 파이프라인 연동 (`tshark`, `tcpdump`)
```bash
# 터미널에서 즉시 패킷 헤더 확인
kcap my-pod -o - | tshark -r -

# 특정 프로토콜 분석
kcap my-pod -o - | tshark -r - -Y "http"
```

### 5. 명시적 모드 및 디버그 이미지 선택
```bash
# Ephemeral Container 모드 명시 (기본값 auto에서도 자동 적용됨)
kcap my-pod --mode ephemeral --image nicolaka/netshoot:latest

# Pod 내부에 이미 tcpdump가 설치되어 있는 경우 초고속 직접 실행
kcap my-pod --mode direct
```

---

## ⚙️ 전체 CLI 플래그 (Flags)

| 플래그 | 단축키 | 기본값 | 설명 |
| :--- | :--- | :--- | :--- |
| `--pod` | `-p` | (인자 전달) | 타겟 Pod 이름 |
| `--container` | `-c` | 첫 번째 컨테이너 | 타겟 컨테이너 이름 |
| `--namespace` | `-n` | 현재 context 네임스페이스 | Kubernetes 네임스페이스 |
| `--interface` | `-i` | `any` | 캡처할 네트워크 인터페이스 (`eth0`, `any` 등) |
| `--filter` | `-f` | `""` | tcpdump BPF 필터 식 (예: `'tcp port 80'`) |
| `--output` | `-o` | Wireshark 실행 | 출력 대상: `.pcap` 파일 경로, `-` (stdout), 미지정 시 Wireshark 실행 |
| `--mode` | `-m` | `auto` | 캡처 모드: `auto`, `ephemeral`, `direct` |
| `--image` | | `nicolaka/netshoot:latest` | Ephemeral 디버그 컨테이너 이미지 |
| `--wireshark-path`| | 자동 탐색 | 로컬 Wireshark 바이너리 경로 수동 지정 |
| `--kubeconfig` | | `~/.kube/config` | 사용할 kubeconfig 파일 경로 |
| `--context` | | 현재 context | 사용할 kubeconfig context 이름 |
| `--verbose` | `-v` | `false` | 상세 디버그 로그 출력 |

---

## 🛠️ 지원 환경 및 요구사항
- **Kubernetes 클러스터**: GKE, EKS, AKS, 온프레미스 (k3s, kubeadm, RKE2, Talos 등) K8s v1.23+
- **컨테이너 런타임**: containerd (모든 버전), CRI-O, Docker
- **로컬 머신**: macOS, Linux, Windows (Wireshark 설치 권장)
