# Topo42

dn42 WireGuard 拓扑面板。Controller 提供 Web UI，Agent 上报本机 `dn42_dummy` IP、`dn42_xx00` 接口和链路探测结果。

## 命名

节点名和 WireGuard 接口名都必须匹配：

```text
dn42_de01
```

格式是 `dn42_` + 两位小写字母 + 两位数字。接口名就是对端节点名。

## ICMP 探测

Controller 把已知节点的 `dn42_dummy` IP 发给 Agent。Agent 只在“对端 dummy IP 小于本机 dummy IP”时发起探测，避免两端同时探测。

探测目标使用对端 dummy IPv6 派生出的链路本地地址：

```text
fd6a:93d4:3358::35 -> fe80::93d4:3358:35
```

Agent 直接发送 ICMPv6 Echo，统计 10 次探测的平均 RTT 和丢包率。

Agent 的 Controller 地址传 `ws://` 或 `wss://`。Controller 设置 `--agent-token` 后，Agent 第三个参数必须传同一个 token。

## 构建

需要 Go 1.22+。

```bash
scripts/build-binary.sh controller
scripts/build-binary.sh agent
```
