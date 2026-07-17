const TOPOLOGY_WIDTH = 1280;
const TOPOLOGY_HEIGHT = 700;
const NODE_WIDTH = 154;
const NODE_HEIGHT = 68;
const EDGE_LABEL_CHARACTER_WIDTH = 6.5;
const EDGE_LABEL_HEIGHT = 18;
const EDGE_LABEL_NODE_GAP = 6;
const EDGE_LABEL_FRACTIONS = [0.52, 0.44, 0.6, 0.36, 0.68, 0.28, 0.76];
const EDGE_LABEL_OFFSETS = [14, -14, 30, -30, 48, -48, 66, -66, 84, -84];

let topology = { nodes: [], edges: [] };
let selectedNodeName = null;
let error = "";

const root = document.getElementById("root");

function linkMetricLabel(latency, packetLoss) {
  const latencyText = latency == null ? "-" : `${latency < 10 ? latency.toFixed(1) : Math.round(latency)} ms`;
  const lossText = packetLoss == null ? "-" : `${packetLoss < 10 ? packetLoss.toFixed(1) : Math.round(packetLoss)}%`;
  return `${latencyText} / ${lossText}`;
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

function boxesOverlap(first, second) {
  return first.left < second.right && first.right > second.left && first.top < second.bottom && first.bottom > second.top;
}

function edgeLabelBounds(x, y, text) {
  const width = text.length * EDGE_LABEL_CHARACTER_WIDTH;
  return {
    left: x - width / 2,
    right: x + width / 2,
    top: y - EDGE_LABEL_HEIGHT + 4,
    bottom: y + 4,
  };
}

function edgeLabelPosition(anchor, peer, text, occupied) {
  const dx = peer.x - anchor.x;
  const dy = peer.y - anchor.y;
  const length = Math.hypot(dx, dy) || 1;

  for (const offset of EDGE_LABEL_OFFSETS) {
    for (const fraction of EDGE_LABEL_FRACTIONS) {
      const x = anchor.x + dx * fraction + (-dy / length) * offset;
      const y = anchor.y + dy * fraction + (dx / length) * offset;
      const bounds = edgeLabelBounds(x, y, text);
      if (bounds.left < 0 || bounds.right > TOPOLOGY_WIDTH || bounds.top < 0 || bounds.bottom > TOPOLOGY_HEIGHT) continue;
      if (!occupied.some((box) => boxesOverlap(bounds, box))) return { x, y, bounds };
    }
  }

  // ponytail: 固定回退仅保证标签可见；真实拓扑耗尽候选时再增加碰撞排序。
  const x = anchor.x + dx * 0.52;
  const y = anchor.y + dy * 0.52;
  return { x, y, bounds: edgeLabelBounds(x, y, text) };
}

function renderTopologyCanvas(positions) {
  if (topology.nodes.length === 0) {
    return '<div class="topologyCanvas"><div class="empty">暂无节点。Agent 连接后会显示 dn42 拓扑。</div></div>';
  }
  const occupied = Object.values(positions).map((position) => ({
    left: position.x - NODE_WIDTH / 2 - EDGE_LABEL_NODE_GAP,
    right: position.x + NODE_WIDTH / 2 + EDGE_LABEL_NODE_GAP,
    top: position.y - NODE_HEIGHT / 2 - EDGE_LABEL_NODE_GAP,
    bottom: position.y + NODE_HEIGHT / 2 + EDGE_LABEL_NODE_GAP,
  }));
  const edgeLines = [];
  const edgeLabels = [];
  topology.edges.forEach((edge) => {
    const source = positions[edge.local_node_name];
    const target = positions[edge.peer_node_name];
    if (!source || !target) return;
    const packetLoss = edge.packet_loss_percent;
    const related = selectedNodeName === edge.local_node_name || selectedNodeName === edge.peer_node_name;
    const classes = `topologyEdge ${edge.connected ? "healthy" : "down"}${related ? " related" : ""}${selectedNodeName && !related ? " dimmed" : ""}`;
    edgeLines.push(`<g class="${classes}"><line x1="${source.x}" y1="${source.y}" x2="${target.x}" y2="${target.y}"></line></g>`);
    if (related) {
      const labelText = linkMetricLabel(edge.latency_ms, packetLoss);
      if (labelText !== "- / -") {
        const labelAnchor = selectedNodeName === edge.local_node_name ? source : target;
        const labelPeer = selectedNodeName === edge.local_node_name ? target : source;
        const label = edgeLabelPosition(labelAnchor, labelPeer, labelText, occupied);
        occupied.push(label.bounds);
        edgeLabels.push(`<text class="topologyEdgeLabel${packetLoss > 0 ? " lossy" : ""}" x="${label.x}" y="${label.y}">${esc(labelText)}</text>`);
      }
    }
  });
  const nodes = topology.nodes.map((node) => {
    const position = positions[node.name] || { x: TOPOLOGY_WIDTH / 2, y: TOPOLOGY_HEIGHT / 2 };
    return `<g class="topologyNode${node.online ? " online" : ""}${node.name === selectedNodeName ? " selected" : ""}" data-node="${esc(node.name)}" transform="translate(${position.x - NODE_WIDTH / 2} ${position.y - NODE_HEIGHT / 2})">
      <rect width="${NODE_WIDTH}" height="${NODE_HEIGHT}" rx="8"></rect>
      <rect class="nodeAccent" width="4" height="${NODE_HEIGHT}" rx="2"></rect>
      <text class="nodeTitle" x="12" y="20">${esc(node.name)}</text>
      <circle class="nodeStatus" cx="${NODE_WIDTH - 15}" cy="17" r="5"></circle>
      <text class="nodeSub" x="12" y="40">${esc(node.node_ips[0] || "-")}</text>
      ${node.node_ips[1] ? `<text class="nodeSub" x="12" y="55">${esc(node.node_ips[1])}</text>` : ""}
    </g>`;
  }).join("");
  return `<div class="topologyCanvas">
    <svg class="topologySvg" viewBox="0 0 ${TOPOLOGY_WIDTH} ${TOPOLOGY_HEIGHT}" role="img">${edgeLines.join("")}${edgeLabels.join("")}${nodes}</svg>
  </div>`;
}

function render() {
  const nodes = topology.nodes;
  const selectedNode = nodes.find((node) => node.name === selectedNodeName) || null;
  const selectedInterfaces = selectedNode?.interfaces || [];
  const onlineCount = nodes.filter((node) => node.online).length;
  const runningEdgeCount = topology.edges.filter((edge) => edge.connected).length;
  // ponytail: linear lookups suit this small topology; index nodes and edges if it grows.
  const interfaces = selectedInterfaces.map((item) => {
    const peerNodeIPs = nodes.find((node) => node.name === item.name)?.node_ips || [];
    const ipv4 = peerNodeIPs.filter((ip) => ip.includes(".")).join(", ");
    const ipv6 = peerNodeIPs.filter((ip) => ip.includes(":")).join(", ");
    const edge = topology.edges.find((edge) => (
      (edge.local_node_name === selectedNode.name && edge.peer_node_name === item.name) ||
      (edge.peer_node_name === selectedNode.name && edge.local_node_name === item.name)
    ));
    return `<article class="statusRow">
      <div>
        <strong>${esc(item.name)}</strong>
        <small>对端 IPv4: ${esc(ipv4 || "-")}</small>
        ${ipv6 ? `<small>对端 IPv6: ${esc(ipv6)}</small>` : ""}
        <small>RTT/丢包: ${esc(linkMetricLabel(edge?.latency_ms ?? item.latency_ms, edge?.packet_loss_percent ?? item.packet_loss_percent))}</small>
      </div>
      <em class="statusPill ${edge?.connected ? "healthy" : "unknown"}">${edge?.connected ? "已连接" : "离线"}</em>
    </article>`;
  }).join("");

  root.innerHTML = `<section class="nodeBoard">
      ${error ? `<div class="formError" role="alert">${esc(error)}</div>` : ""}
      <div class="summaryGrid">
        <div class="statCard"><span>节点</span><strong>${onlineCount}/${nodes.length}</strong><small>在线</small></div>
        <div class="statCard"><span>接口</span><strong>${selectedInterfaces.length}</strong><small>${esc(selectedNode?.name || "当前节点")}</small></div>
        <div class="statCard"><span>链路</span><strong>${runningEdgeCount}/${topology.edges.length}</strong><small>已连接</small></div>
      </div>
      <section class="topologyPanel">
        <h2>网络拓扑</h2>
        ${renderTopologyCanvas(nodePositions(nodes))}
      </section>
      <section class="detailPanel">
        <h2>连接状态</h2>
        ${selectedNode ? `<div class="nodeSnapshot">
          <div class="snapshotHeader"><div><strong>${esc(selectedNode.name)}</strong></div><em class="statusPill ${selectedNode.online ? "healthy" : "unknown"}">${selectedNode.online ? "在线" : "离线"}</em></div>
          <dl class="infoGrid">
            <div><dt>Agent</dt><dd>${esc(selectedNode.agent_version || "-")}</dd></div>
            <div><dt>最后心跳</dt><dd>${esc(selectedNode.last_seen_at ? new Date(selectedNode.last_seen_at).toLocaleString() : "-")}</dd></div>
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
  } catch (err) {
    error = err instanceof Error ? err.message : String(err);
  } finally {
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
  }
});

render();
void refreshAll();
setInterval(() => void refreshAll(), 5000);
