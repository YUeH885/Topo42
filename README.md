# Topo42

dn42 Babel 拓扑面板。Controller 提供 Web UI，Agent 上报本机 `dn42_dummy` IP，以及从 `babeld` 读取的接口和链路指标。

## Babel 采集

Agent 每 30 秒连接一次 `babeld` 只读本地协议，读取已配置的接口、邻居平滑 RTT 和 Hello 接收历史。丢包率来自 `reach`/`ureach` 位图，是 Babel Hello 的短窗口估值，不是 ICMP 丢包率。

`babeld` 至少需要配置只读本地端口，并在参与采集的接口上启用时间戳：

```text
local-port 33123
interface wg-peer type tunnel enable-timestamps true
```

Agent 默认连接 `[::1]:33123`。也可以传 TCP 地址或 Unix socket 路径：

```bash
topo42-agent --babel-address '[::1]:33123' ws://127.0.0.1:8000 dn42_cn01 change-me
topo42-agent --babel-address /run/babeld.sock ws://127.0.0.1:8000 dn42_cn01 change-me
```

所有选项必须放在位置参数之前。Agent 的 Controller 地址传 `ws://` 或 `wss://`。Controller 设置 `--agent-token` 后，最后一个位置参数必须传同一个 token。

## 构建

需要 Go 1.22+。

```bash
scripts/build-binary.sh
```
