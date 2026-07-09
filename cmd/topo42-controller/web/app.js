const TOPOLOGY_WIDTH = 1280;
const TOPOLOGY_HEIGHT = 700;
const NODE_WIDTH = 154;
const NODE_HEIGHT = 68;

let topology = { nodes: [], edges: [] };
let selectedNodeName = null;
let loading = true;
let error = "";
let updatedAt = null;

const root = document.getElementById("root");

function formatDate(value) {
  return value ? new Date(value).toLocaleString() : "-";
}

function linkMetricLabel(latency, packetLoss) {
  const latencyText = latency == null ? "-" : `${latency < 10 ? latency.toFixed(1) : Math.round(latency)} ms`;
  const lossText = packetLoss == null ? "-" : `${packetLoss < 10 ? packetLoss.toFixed(1) : Math.round(packetLoss)}%`;
  return `${latencyText} / ${lossText}`;
}

function statusText(status) {
  return { running: "已连接", online: "在线", offline: "离线" }[status] || "未知";
}

function interfaceLinkMetrics(nodeName, item) {
  const edge = topology.edges.find((edge) => (
    (edge.local_node_name === nodeName && edge.local_interface_name === item.name) ||
    (edge.peer_node_name === nodeName && edge.peer_interface_name === item.name)
  ));
  if (!edge) return [item.latency_ms, item.packet_loss_percent];
  if (edge.local_node_name === nodeName) {
    return [item.latency_ms ?? edge.peer_latency_ms, item.packet_loss_percent ?? edge.peer_packet_loss_percent];
  }
  return [item.latency_ms ?? edge.local_latency_ms, item.packet_loss_percent ?? edge.local_packet_loss_percent];
}

function esc(value) {
  return String(value ?? "").replace(/[&<>"']/g, (char) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    "\"": "&quot;",
    "'": "&#39;",
  }[char]));
}

function nodePositions(nodes) {
  const count = Math.max(nodes.length, 1);
  const radiusX = Math.max(180, Math.min(500, count * 54));
  const radiusY = Math.max(150, Math.min(260, count * 30));
  return Object.fromEntries(nodes.map((node, index) => {
    const angle = (Math.PI * 2 * index) / count - Math.PI / 2;
    return [node.name, {
      x: TOPOLOGY_WIDTH / 2 + Math.cos(angle) * radiusX,
      y: TOPOLOGY_HEIGHT / 2 + Math.sin(angle) * radiusY,
    }];
  }));
}

function renderTopologyCanvas(positions) {
  if (topology.nodes.length === 0) {
    return '<div class="topologyCanvas"><div class="empty">暂无节点。Agent 连接后会显示 dn42 拓扑。</div></div>';
  }
  const edgeParts = [...topology.edges].sort((a, b) => {
    const aRelated = Number(selectedNodeName === a.local_node_name || selectedNodeName === a.peer_node_name);
    const bRelated = Number(selectedNodeName === b.local_node_name || selectedNodeName === b.peer_node_name);
    return aRelated - bRelated;
  }).map((edge) => {
    const source = positions[edge.local_node_name];
    const target = positions[edge.peer_node_name];
    if (!source || !target) return { line: "", label: "" };
    const running = edge.local_status === "running" && edge.peer_status === "running";
    const related = selectedNodeName === edge.local_node_name || selectedNodeName === edge.peer_node_name;
    const classes = `topologyEdge ${running ? "healthy" : "down"}${related ? " related" : ""}${selectedNodeName && !related ? " dimmed" : ""}`;
    let label = "";
    if (related) {
      const labelText = linkMetricLabel(edge.local_latency_ms ?? edge.peer_latency_ms, edge.local_packet_loss_percent ?? edge.peer_packet_loss_percent);
      if (labelText !== "- / -") {
        const labelAnchor = selectedNodeName === edge.local_node_name ? source : target;
        const labelPeer = selectedNodeName === edge.local_node_name ? target : source;
        const dx = target.x - source.x;
        const dy = target.y - source.y;
        const length = Math.hypot(dx, dy) || 1;
        const labelX = labelAnchor.x + (labelPeer.x - labelAnchor.x) * 0.52 + (-dy / length) * 14;
        const labelY = labelAnchor.y + (labelPeer.y - labelAnchor.y) * 0.52 + (dx / length) * 14;
        label = `<text class="topologyEdgeLabel" x="${labelX}" y="${labelY}">${esc(labelText)}</text>`;
      }
    }
    return {
      line: `<g class="${classes}"><line x1="${source.x}" y1="${source.y}" x2="${target.x}" y2="${target.y}"></line></g>`,
      label,
    };
  });
  const edges = edgeParts.map((edge) => edge.line).join("");
  const edgeLabels = edgeParts.map((edge) => edge.label).join("");
  const nodes = topology.nodes.map((node) => {
    const position = positions[node.name] || { x: TOPOLOGY_WIDTH / 2, y: TOPOLOGY_HEIGHT / 2 };
    const online = node.status === "online";
    return `<g class="topologyNode${online ? " online" : ""}${node.name === selectedNodeName ? " selected" : ""}" data-node="${esc(node.name)}" transform="translate(${position.x - NODE_WIDTH / 2} ${position.y - NODE_HEIGHT / 2})">
      <rect width="${NODE_WIDTH}" height="${NODE_HEIGHT}" rx="8"></rect>
      <rect class="nodeAccent" width="4" height="${NODE_HEIGHT}" rx="2"></rect>
      <text class="nodeTitle" x="12" y="20">${esc(node.name)}</text>
      <circle class="nodeStatus" cx="${NODE_WIDTH - 15}" cy="17" r="5"></circle>
      <text class="nodeSub" x="12" y="40">${esc(node.node_ips[0] || "-")}</text>
      ${node.node_ips[1] ? `<text class="nodeSub" x="12" y="55">${esc(node.node_ips[1])}</text>` : ""}
    </g>`;
  }).join("");
  return `<div class="topologyCanvas">
    <svg class="topologySvg" viewBox="0 0 ${TOPOLOGY_WIDTH} ${TOPOLOGY_HEIGHT}" role="img">${edges}${nodes}${edgeLabels}</svg>
  </div>`;
}

function render() {
  const nodes = topology.nodes;
  const selectedNode = nodes.find((node) => node.name === selectedNodeName) || null;
  const selectedInterfaces = selectedNode?.interfaces || [];
  const onlineCount = nodes.filter((node) => node.status === "online").length;
  const runningEdgeCount = topology.edges.filter((edge) => edge.local_status === "running" && edge.peer_status === "running").length;
  const positions = nodePositions(nodes);
  const interfaces = selectedInterfaces.map((item) => {
    const ipv4 = item.peer_node_ips.filter((ip) => ip.includes(".")).join(", ");
    const ipv6 = item.peer_node_ips.filter((ip) => ip.includes(":")).join(", ");
    const [latency, packetLoss] = interfaceLinkMetrics(selectedNode.name, item);
    return `<article class="statusRow">
      <div>
        <strong>${esc(item.name)}</strong>
        <small>对端 IPv4: ${esc(ipv4 || "-")}</small>
        ${ipv6 ? `<small>对端 IPv6: ${esc(ipv6)}</small>` : ""}
        <small>RTT/丢包: ${esc(linkMetricLabel(latency, packetLoss))}</small>
        <small>对端: ${esc(item.name)}</small>
      </div>
      <em class="statusPill ${item.runtime_status === "running" ? "healthy" : "unknown"}">${esc(statusText(item.runtime_status))}</em>
    </article>`;
  }).join("");

  root.innerHTML = `<section class="nodeBoard">
      ${error ? `<div class="formError" role="alert">${esc(error)}</div>` : ""}
      <div class="summaryGrid">
        <div class="statCard"><span>节点</span><strong>${onlineCount}/${nodes.length}</strong><small>在线</small></div>
        <div class="statCard"><span>接口</span><strong>${selectedInterfaces.length}</strong><small>${esc(selectedNode?.name || "当前节点")}</small></div>
        <div class="statCard"><span>链路</span><strong>${runningEdgeCount}/${topology.edges.length}</strong><small>已连接</small></div>
        <div class="statCard"><span>刷新</span><strong>${loading ? "..." : esc(formatDate(updatedAt))}</strong><small>每 5 秒自动刷新</small></div>
      </div>
      <section class="topologyPanel">
        <div class="topologyHeader">
          <h2>网络拓扑</h2>
          <div class="topologyToolbar">
            <span class="topologyMeta">${topology.nodes.length} 节点 / ${topology.edges.length} 链路</span>
            <button class="iconButton" id="refreshButton" title="刷新">刷新</button>
          </div>
        </div>
        ${renderTopologyCanvas(positions)}
      </section>
      <section class="detailPanel">
        <h2>连接状态</h2>
        ${selectedNode ? `<div class="nodeSnapshot">
          <div class="snapshotHeader"><div><strong>${esc(selectedNode.name)}</strong></div><em class="statusPill ${selectedNode.status === "online" ? "healthy" : "unknown"}">${esc(statusText(selectedNode.status))}</em></div>
          <dl class="infoGrid">
            <div><dt>Agent</dt><dd>${esc(selectedNode.agent_version || "-")}</dd></div>
            <div><dt>最后心跳</dt><dd>${esc(formatDate(selectedNode.last_seen_at))}</dd></div>
            <div><dt>节点 IP</dt><dd>${esc(selectedNode.node_ips.join(", ") || "-")}</dd></div>
          </dl>
        </div>` : '<div class="empty">在网络拓扑中选择一个节点查看状态。</div>'}
        <div class="sectionBlock">
          <h3>接口</h3>
          <div class="statusList">${interfaces}${selectedNode && selectedInterfaces.length === 0 ? '<div class="empty">当前节点未检测到 dn42 接口。</div>' : ""}</div>
        </div>
      </section>
    </section>`;
}

async function refreshAll() {
  error = "";
  try {
    const response = await fetch("/api/topology", { headers: { Accept: "application/json" } });
    if (!response.ok) throw new Error(`${response.status}: ${response.statusText}`);
    topology = await response.json();
    if (selectedNodeName != null && !topology.nodes.some((node) => node.name === selectedNodeName)) {
      selectedNodeName = null;
    }
    updatedAt = new Date();
  } catch (err) {
    error = err instanceof Error ? err.message : String(err);
  } finally {
    loading = false;
    render();
  }
}

root.addEventListener("click", (event) => {
  const target = event.target instanceof Element ? event.target : null;
  if (!target) return;
  const node = target.closest(".topologyNode");
  if (node) {
    selectedNodeName = node.dataset.node;
    render();
    return;
  }
  if (target.closest(".topologySvg")) {
    selectedNodeName = null;
    render();
    return;
  }
  if (target.closest("#refreshButton")) {
    void refreshAll();
  }
});

render();
void refreshAll();
setInterval(() => void refreshAll(), 5000);
