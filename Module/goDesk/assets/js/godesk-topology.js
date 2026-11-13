/* =========================================================
   goDesk — editor visual da dependência v2 (arquitetura)
   Página: godesk.dependencies.v2.edit (só roda se #gd-topo existir)

   Estado em JSON (mesmo schema do godesk-dependencies-v2.yaml), desenhado
   em SVG. Modos:
     - Editar:  arrastar hosts, selecionar host/link, Delete remove
     - Ligar:   clique em dois hosts pra criar/remover o link
     - Simular: clique nos hosts pra marcar DOWN e ver quem abriria chamado
                (mesma decisão do internal/app/dependency_v2.go)
   Salvar manda tudo num campo só (topology_json); o PHP valida e grava.
========================================================= */
(function () {
  "use strict";

  const SVGNS = "http://www.w3.org/2000/svg";
  const NODE_H = 34;
  const RACK_COLORS = ["#2f6fdf", "#d9822b", "#2e9e5b", "#9b59b6", "#c0392b", "#16a085", "#7f8c8d"];
  // ordem das camadas ao importar de um grupo (nome do subgrupo, lowercase)
  const LAYER_ORDER = ["router", "switch/core", "firewall", "switch", "rtac", "server"];

  let S = null;
  let svg = null;

  const lc = (s) => String(s == null ? "" : s).trim().toLowerCase();

  function escapeHtml(s) {
    return String(s == null ? "" : s)
      .replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;").replace(/'/g, "&#39;");
  }

  function storageGet(k) { try { return window.sessionStorage.getItem(k); } catch (e) { return null; } }
  function storageSet(k, v) { try { window.sessionStorage.setItem(k, v); } catch (e) { /* sem storage */ } }

  // ---------- modelo ----------

  function newSite(name) {
    const s = {
      name: name,
      root: S.defaults.root,
      down_event: S.defaults.down_event,
      status: { host: "", event: "Powerplant Down" },
      nodes: [],
      links: [],
    };
    s.nodes.push({ host: s.root, rack: "", x: 420, y: 50 });
    return s;
  }

  function prepSite(raw) {
    const s = {
      name: String(raw.name || ""),
      root: String(raw.root || S.defaults.root),
      down_event: String(raw.down_event || S.defaults.down_event),
      status: { host: String((raw.status || {}).host || ""), event: String((raw.status || {}).event || "") },
      nodes: (raw.nodes || []).map((n) => ({
        host: String(n.host || ""), rack: String(n.rack || ""), x: +n.x || 0, y: +n.y || 0,
      })).filter((n) => n.host !== ""),
      links: (raw.links || []).filter((l) => l && l.length === 2).map((l) => [String(l[0]), String(l[1])]),
    };
    // todo host citado num link precisa de um nó (posição) — e o root também
    const ensure = (h) => { if (!nodeOf(s, h)) s.nodes.push({ host: h, rack: "", x: 0, y: 0 }); };
    ensure(s.root);
    s.links.forEach((l) => { ensure(l[0]); ensure(l[1]); });
    if (s.nodes.every((n) => n.x === 0 && n.y === 0) || s.nodes.some((n) => n.x === 0 && n.y === 0 && s.nodes.length > 1)) {
      autoLayout(s, s.nodes.every((n) => n.x === 0 && n.y === 0));
    }
    return s;
  }

  function site() { return S.sites[S.cur] || null; }
  function nodeOf(s, host) { const k = lc(host); return s.nodes.find((n) => lc(n.host) === k) || null; }
  function isRoot(s, host) { return lc(host) === lc(s.root); }
  function nodeWidth(host) { return Math.max(90, String(host).length * 7.2 + 26); }
  function hasLink(s, a, b) {
    const ka = lc(a), kb = lc(b);
    return s.links.findIndex((l) => (lc(l[0]) === ka && lc(l[1]) === kb) || (lc(l[0]) === kb && lc(l[1]) === ka));
  }
  function dirty() { S.dirty = true; }

  function siteOfHost(host) {
    const k = lc(host);
    return S.sites.findIndex((s) => !isRoot(s, host) && (lc(s.status.host) === k || !!nodeOf(s, host)));
  }

  function renameHost(s, oldName, newName) {
    const k = lc(oldName);
    s.nodes.forEach((n) => { if (lc(n.host) === k) n.host = newName; });
    s.links.forEach((l) => { if (lc(l[0]) === k) l[0] = newName; if (lc(l[1]) === k) l[1] = newName; });
    if (S.sim.down.delete(k)) S.sim.down.add(lc(newName));
  }

  function removeNode(s, host) {
    const k = lc(host);
    s.nodes = s.nodes.filter((n) => lc(n.host) !== k);
    s.links = s.links.filter((l) => lc(l[0]) !== k && lc(l[1]) !== k);
    S.sim.down.delete(k);
  }

  // ---------- layout ----------

  // camadas por distância (BFS) a partir do root; sem caminho = última camada
  function autoLayout(s, all) {
    const adj = adjacency(s);
    const level = {};
    const rootK = lc(s.root);
    level[rootK] = 0;
    const q = [rootK];
    while (q.length) {
      const cur = q.shift();
      (adj[cur] || new Set()).forEach((v) => {
        if (level[v] === undefined) { level[v] = level[cur] + 1; q.push(v); }
      });
    }
    let maxL = 0;
    Object.values(level).forEach((l) => { maxL = Math.max(maxL, l); });

    const rows = {};
    s.nodes.forEach((n) => {
      if (!all && !(n.x === 0 && n.y === 0)) return;
      const l = level[lc(n.host)] === undefined ? maxL + 1 : level[lc(n.host)];
      (rows[l] = rows[l] || []).push(n);
    });

    const W = Math.max(900, ...Object.values(rows).map((r) => r.reduce((a, n) => a + nodeWidth(n.host) + 28, 0) + 80));
    const baseY = all ? 50 : Math.max(50, ...s.nodes.filter((n) => !(n.x === 0 && n.y === 0)).map((n) => n.y)) + 100;
    Object.keys(rows).map(Number).sort((a, b) => a - b).forEach((l, i) => {
      const row = rows[l].sort((a, b) => (a.rack || "").localeCompare(b.rack || "") || a.host.localeCompare(b.host));
      const total = row.reduce((a, n) => a + nodeWidth(n.host) + 28, -28);
      let x = (W - total) / 2;
      row.forEach((n) => {
        const w = nodeWidth(n.host);
        n.x = Math.round(x + w / 2);
        n.y = all ? 50 + l * 100 : baseY + i * 100;
        x += w + 28;
      });
    });
  }

  function freeSpot(s) {
    for (let y = 60; y < 4000; y += 90) {
      for (let x = 110; x < 1100; x += 170) {
        if (!s.nodes.some((n) => Math.abs(n.x - x) < 150 && Math.abs(n.y - y) < 60)) return { x, y };
      }
    }
    return { x: 110, y: 60 };
  }

  // ---------- decisão (port do dependency_v2.go) ----------

  function adjacency(s) {
    const adj = {};
    s.links.forEach((l) => {
      const a = lc(l[0]), b = lc(l[1]);
      (adj[a] = adj[a] || new Set()).add(b);
      (adj[b] = adj[b] || new Set()).add(a);
    });
    return adj;
  }

  function reachable(adj, rootK, skip, down) {
    const visited = new Set([rootK]);
    const frontier = new Set();
    const q = [rootK];
    while (q.length) {
      const cur = q.shift();
      (adj[cur] || new Set()).forEach((v) => {
        if (v === skip || visited.has(v)) return;
        if (down.has(v)) { frontier.add(v); return; }
        visited.add(v);
        q.push(v);
      });
    }
    return { visited, frontier };
  }

  function analyze(s, adj, host, down) {
    const h = lc(host);
    const { visited, frontier } = reachable(adj, lc(s.root), h, down);
    for (const v of (adj[h] || [])) if (visited.has(v)) return { root: true, blockers: [] };

    const side = new Set([h]);
    const q = [h];
    while (q.length) {
      const cur = q.shift();
      (adj[cur] || new Set()).forEach((v) => {
        if (visited.has(v) || side.has(v)) return;
        side.add(v);
        if (!frontier.has(v)) q.push(v);
      });
    }
    const blockers = [...frontier].filter((v) => side.has(v)).sort();
    // sem DOWN que explique (topologia desconectada do root): abre chamado
    if (!blockers.length) return { root: true, blockers: [] };
    return { root: false, blockers };
  }

  function nameOf(s, k) { const n = s.nodes.find((x) => lc(x.host) === k); return n ? n.host : k; }

  function evaluate(s, adj, host, down, siteDown) {
    if (s.status.host && lc(host) === lc(s.status.host)) return { hold: false, why: "host de status" };
    if (siteDown && s.status.host) return { hold: true, why: "localidade DOWN (" + (s.status.event || "status") + ")" };
    if (!(adj[lc(host)] || new Set()).size) return { hold: false, why: "fora da topologia (sem links)" };
    const r = analyze(s, adj, host, down);
    if (r.root) return { hold: false, why: "causa raiz" };
    return {
      hold: true,
      why: "cortado por " + r.blockers.map((b) => {
        const n = nodeOf(s, nameOf(s, b));
        return nameOf(s, b) + (n && n.rack ? " (rack " + n.rack + ")" : "");
      }).join(", "),
    };
  }

  function simulate(s) {
    const adj = adjacency(s);
    const down = new Set([...S.sim.down].filter((k) => !!nodeOf(s, k) && !isRoot(s, k)));
    const siteDown = S.sim.siteDown && !!s.status.host;
    const state = {};
    const open = [];
    const held = [];

    if (siteDown) open.push({ host: s.status.host, why: "evento de status ativo" });

    const { visited } = reachable(adj, lc(s.root), null, down);
    s.nodes.forEach((n) => {
      const k = lc(n.host);
      if (isRoot(s, n.host)) return;
      if (down.has(k)) {
        const d = evaluate(s, adj, n.host, down, siteDown);
        state[k] = d.hold ? "held" : "open";
        (d.hold ? held : open).push({ host: n.host, why: d.why });
      }
      else if ((adj[k] || new Set()).size && !visited.has(k)) {
        state[k] = "cut";
      }
    });
    return { state, open, held, siteDown };
  }

  // problemas que contam como DOWN: mesma regra do matchEvent do Go
  function matchEvent(pattern, name) {
    pattern = String(pattern || "").trim();
    name = String(name || "").trim();
    if (pattern === "") return true;
    if (pattern.indexOf("*") < 0) return pattern.toLowerCase() === name.toLowerCase();
    const re = pattern.split("*").map((p) => p.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")).join("[\\s\\S]*");
    return new RegExp("^" + re + "$", "i").test(name);
  }

  // ---------- render ----------

  function el(tag, attrs, parent) {
    const e = document.createElementNS(SVGNS, tag);
    Object.keys(attrs || {}).forEach((k) => e.setAttribute(k, attrs[k]));
    if (parent) parent.appendChild(e);
    return e;
  }

  function rackColors(s) {
    const racks = [...new Set(s.nodes.map((n) => n.rack).filter(Boolean))].sort((a, b) => a.localeCompare(b, undefined, { numeric: true }));
    const map = {};
    racks.forEach((r, i) => { map[r] = RACK_COLORS[i % RACK_COLORS.length]; });
    return map;
  }

  function renderAll() {
    renderSiteSelect();
    renderSiteProps();
    renderModes();
    renderCanvas();
    renderSide();
  }

  function renderSiteSelect() {
    const sel = document.getElementById("gd-topo-site");
    sel.innerHTML = S.sites.length
      ? S.sites.map((s, i) => `<option value="${i}"${i === S.cur ? " selected" : ""}>${escapeHtml(s.name || "(sem nome)")}</option>`).join("")
      : '<option value="">— nenhuma —</option>';
  }

  function renderSiteProps() {
    const s = site();
    document.querySelectorAll("[data-gd-site-field]").forEach((inp) => {
      const f = inp.dataset.gdSiteField;
      inp.disabled = !s;
      if (document.activeElement === inp) return;
      inp.value = !s ? "" : f === "status.host" ? s.status.host : f === "status.event" ? s.status.event : s[f];
    });
  }

  function renderModes() {
    document.querySelectorAll("[data-gd-topo-mode]").forEach((b) => {
      b.classList.toggle("gd-btn-primary", b.dataset.gdTopoMode === S.mode);
    });
    const bar = document.querySelector(".gd-topo-simbar");
    bar.style.display = S.mode === "sim" ? "" : "none";
    const s = site();
    const chk = document.getElementById("gd-topo-sim-sitedown");
    chk.checked = S.sim.siteDown;
    chk.disabled = !s || !s.status.host;
    document.querySelector(".gd-topo-sim-event").textContent = (s && s.status.event) || "qualquer problema do host de status";
    document.getElementById("gd-topo").classList.toggle("gd-topo-linking", S.mode === "link");
  }

  function renderCanvas() {
    const s = site();
    while (svg.firstChild) svg.removeChild(svg.firstChild);
    const wrap = svg.parentNode;

    if (!s) {
      svg.setAttribute("width", Math.max(600, wrap.clientWidth - 2));
      svg.setAttribute("height", 200);
      const t = el("text", { x: 20, y: 40, class: "gd-topo-empty" }, svg);
      t.textContent = "Nenhuma localidade cadastrada — clique em “+ Nova localidade”.";
      renderLegend(null, {});
      return;
    }

    const maxX = Math.max(0, ...s.nodes.map((n) => n.x + nodeWidth(n.host) / 2));
    const maxY = Math.max(0, ...s.nodes.map((n) => n.y));
    svg.setAttribute("width", Math.max(wrap.clientWidth - 2, maxX + 80));
    svg.setAttribute("height", Math.max(560, maxY + 90));

    const sim = S.mode === "sim" ? simulate(s) : null;
    const colors = rackColors(s);

    // host de status (fora do grafo)
    if (s.status.host) {
      const g = el("g", { class: "gd-topo-status" + (sim && sim.siteDown ? " is-down" : ""), transform: "translate(12,12)" }, svg);
      const label = "Status: " + s.status.host + (s.status.event ? " — " + s.status.event : "");
      el("rect", { x: 0, y: 0, rx: 6, width: label.length * 6.6 + 20, height: 24 }, g);
      const t = el("text", { x: 10, y: 16 }, g);
      t.textContent = label;
    }

    const links = el("g", { class: "gd-topo-links" }, svg);
    s.links.forEach((l, i) => {
      const a = nodeOf(s, l[0]), b = nodeOf(s, l[1]);
      if (!a || !b) return;
      let cls = "gd-topo-link";
      if (S.sel && S.sel.type === "link" && S.sel.idx === i) cls += " is-selected";
      if (sim) {
        const sa = sim.state[lc(a.host)], sb = sim.state[lc(b.host)];
        if (sa || sb) cls += " is-cut";
      }
      el("line", { x1: a.x, y1: a.y, x2: b.x, y2: b.y, class: cls }, links);
      el("line", { x1: a.x, y1: a.y, x2: b.x, y2: b.y, class: "gd-topo-link-hit", "data-link": i }, links);
    });

    const nodes = el("g", { class: "gd-topo-nodes" }, svg);
    s.nodes.forEach((n) => {
      const k = lc(n.host);
      const w = nodeWidth(n.host);
      let cls = "gd-topo-node";
      if (isRoot(s, n.host)) cls += " is-root";
      if (S.sel && S.sel.type === "node" && lc(S.sel.host) === k) cls += " is-selected";
      if (S.linkFrom && lc(S.linkFrom) === k) cls += " is-linkfrom";
      if (sim && sim.state[k]) cls += " sim-" + sim.state[k];
      if (!isRoot(s, n.host) && !(adjacency(s)[k] || new Set()).size) cls += " is-isolated";

      const g = el("g", { class: cls, transform: `translate(${n.x},${n.y})`, "data-host": n.host }, nodes);
      const title = el("title", {}, g);
      title.textContent = n.host + (n.rack ? " — rack " + n.rack : "");
      el("rect", { x: -w / 2, y: -NODE_H / 2, width: w, height: NODE_H, rx: isRoot(s, n.host) ? NODE_H / 2 : 7, class: "gd-topo-box" }, g);
      if (n.rack && colors[n.rack]) {
        el("rect", { x: -w / 2, y: -NODE_H / 2, width: 7, height: NODE_H, rx: 3, fill: colors[n.rack], class: "gd-topo-rack" }, g);
      }
      const t = el("text", { x: n.rack ? 3 : 0, y: 4, "text-anchor": "middle" }, g);
      t.textContent = n.host;
    });

    renderLegend(s, colors);
  }

  function renderLegend(s, colors) {
    const box = document.getElementById("gd-topo-legend");
    let html = "";
    Object.keys(colors).forEach((r) => {
      html += `<span class="gd-topo-legend-item"><i style="background:${colors[r]}"></i>Rack ${escapeHtml(r)}</span>`;
    });
    if (S.mode === "sim") {
      html += '<span class="gd-topo-legend-item"><i class="sim-open"></i>DOWN — abre chamado</span>';
      html += '<span class="gd-topo-legend-item"><i class="sim-held"></i>DOWN — segurado</span>';
      html += '<span class="gd-topo-legend-item"><i class="sim-cut"></i>sem caminho (cairia junto)</span>';
    }
    else if (s) {
      html += '<span class="gd-topo-legend-item"><i class="is-isolated"></i>sem links (a v2 não gerencia)</span>';
    }
    box.innerHTML = html;
  }

  function linkListHtml(s, host) {
    const k = lc(host);
    const items = [];
    s.links.forEach((l, i) => {
      if (lc(l[0]) !== k && lc(l[1]) !== k) return;
      const other = lc(l[0]) === k ? l[1] : l[0];
      items.push(`<li>${escapeHtml(other)} <button type="button" class="gd-btn gd-btn-danger gd-btn-xs" data-gd-topo-unlink="${i}">✕</button></li>`);
    });
    return items.length ? `<ul class="gd-topo-linklist">${items.join("")}</ul>` : '<div class="gd-muted">Sem links.</div>';
  }

  function renderSide() {
    const box = document.getElementById("gd-topo-side");
    const s = site();
    if (!s) { box.innerHTML = ""; return; }

    if (S.mode === "sim") {
      const sim = simulate(s);
      const li = (x) => `<li><b>${escapeHtml(x.host)}</b><br><span class="gd-muted">${escapeHtml(x.why)}</span></li>`;
      box.innerHTML =
        `<div class="gd-small-title">Abririam chamado (${sim.open.length})</div>` +
        (sim.open.length ? `<ul class="gd-topo-res gd-topo-res-open">${sim.open.map(li).join("")}</ul>` : '<div class="gd-muted">Nenhum.</div>') +
        `<div class="gd-small-title" style="margin-top:12px;">Segurados (${sim.held.length})</div>` +
        (sim.held.length ? `<ul class="gd-topo-res gd-topo-res-held">${sim.held.map(li).join("")}</ul>` : '<div class="gd-muted">Nenhum.</div>') +
        '<div class="gd-muted" style="margin-top:12px;">A simulação não altera nada. Na hora do alerta o goDesk faz a mesma conta com o estado real do Zabbix.</div>';
      return;
    }

    if (S.sel && S.sel.type === "node") {
      const n = nodeOf(s, S.sel.host);
      if (n) {
        const root = isRoot(s, n.host);
        box.innerHTML =
          `<div class="gd-small-title">${root ? "Root (entrada do Zabbix)" : "Host"}</div>` +
          (root
            ? `<div><b>${escapeHtml(n.host)}</b></div><div class="gd-muted">Ponto de partida do caminho — não é host do Zabbix. Renomeie no campo “Root”.</div>`
            : `<div class="gd-field"><label>Nome técnico</label><input type="text" id="gd-topo-sel-host" list="gd-topo-hosts" value="${escapeHtml(n.host)}"></div>` +
              `<div class="gd-field" style="margin-top:8px;"><label>Rack</label><input type="text" id="gd-topo-sel-rack" value="${escapeHtml(n.rack)}" placeholder="ex: 1"></div>`) +
          `<div class="gd-small-title" style="margin-top:12px;">Ligado a</div>${linkListHtml(s, n.host)}` +
          (root ? "" : '<div class="gd-actions"><button type="button" class="gd-btn gd-btn-danger" data-gd-topo="node-remove">Remover host</button></div>');
        return;
      }
    }

    if (S.sel && S.sel.type === "link" && s.links[S.sel.idx]) {
      const l = s.links[S.sel.idx];
      box.innerHTML =
        '<div class="gd-small-title">Link</div>' +
        `<div><b>${escapeHtml(l[0])}</b> ↔ <b>${escapeHtml(l[1])}</b></div>` +
        `<div class="gd-actions"><button type="button" class="gd-btn gd-btn-danger" data-gd-topo-unlink="${S.sel.idx}">Remover link</button></div>`;
      return;
    }

    // visão geral + avisos
    const adj = adjacency(s);
    const isolated = s.nodes.filter((n) => !isRoot(s, n.host) && !(adj[lc(n.host)] || new Set()).size).map((n) => n.host);
    const { visited } = reachable(adj, lc(s.root), null, new Set());
    const disconnected = s.nodes.filter((n) => !isRoot(s, n.host) && (adj[lc(n.host)] || new Set()).size && !visited.has(lc(n.host))).map((n) => n.host);
    let warn = "";
    if (!(adj[lc(s.root)] || new Set()).size && s.links.length) warn += '<div class="gd-banner gd-err">Nenhum link sai do root — ligue o root aos equipamentos de entrada (ex: SDs).</div>';
    if (disconnected.length) warn += `<div class="gd-banner gd-warn">Sem caminho até o root: ${escapeHtml(disconnected.join(", "))}</div>`;
    if (isolated.length) warn += `<div class="gd-banner gd-warn">Sem links (não gerenciados pela topologia): ${escapeHtml(isolated.join(", "))}</div>`;
    if (!s.status.host) warn += '<div class="gd-banner gd-warn">Sem host de status: a regra “localidade DOWN” fica desligada.</div>';

    box.innerHTML =
      `<div class="gd-small-title">${escapeHtml(s.name || "(sem nome)")}</div>` +
      `<div class="gd-muted">${s.nodes.length - 1} hosts · ${s.links.length} links</div>` + warn +
      '<div class="gd-muted" style="margin-top:10px;"><b>Editar</b>: arraste os hosts; clique para selecionar; <kbd>Delete</kbd> remove.<br>' +
      '<b>Ligar</b>: clique num host e depois em outro (clicar de novo num link existente remove). O primeiro host fica marcado para ligar vários em sequência.<br>' +
      '<b>Simular</b>: clique nos hosts para marcar DOWN.</div>';
  }

  // ---------- interação no canvas ----------

  function svgPoint(evt) {
    const pt = svg.createSVGPoint();
    pt.x = evt.clientX;
    pt.y = evt.clientY;
    return pt.matrixTransform(svg.getScreenCTM().inverse());
  }

  function onPointerDown(e) {
    const s = site();
    if (!s) return;
    const g = e.target.closest(".gd-topo-node");
    const hit = e.target.closest(".gd-topo-link-hit");

    if (g) {
      const host = g.getAttribute("data-host");
      if (S.mode === "sim") {
        if (isRoot(s, host)) return;
        const k = lc(host);
        if (S.sim.down.has(k)) S.sim.down.delete(k); else S.sim.down.add(k);
        renderCanvas(); renderSide();
        return;
      }
      if (S.mode === "link") {
        if (!S.linkFrom) { S.linkFrom = host; }
        else if (lc(S.linkFrom) === lc(host)) { S.linkFrom = null; }
        else {
          const idx = hasLink(s, S.linkFrom, host);
          if (idx >= 0) s.links.splice(idx, 1); else s.links.push([S.linkFrom, host]);
          dirty();
        }
        renderCanvas(); renderSide();
        return;
      }
      const n = nodeOf(s, host);
      const p = svgPoint(e);
      S.drag = { host, dx: p.x - n.x, dy: p.y - n.y, moved: false };
      svg.setPointerCapture(e.pointerId);
      e.preventDefault();
      return;
    }

    if (hit && S.mode === "edit") {
      S.sel = { type: "link", idx: +hit.getAttribute("data-link") };
      renderCanvas(); renderSide();
      return;
    }

    S.sel = null;
    S.linkFrom = null;
    renderCanvas(); renderSide();
  }

  function onPointerMove(e) {
    if (!S.drag) return;
    const s = site();
    const n = nodeOf(s, S.drag.host);
    if (!n) return;
    const p = svgPoint(e);
    const x = Math.max(nodeWidth(n.host) / 2 + 4, Math.round((p.x - S.drag.dx) / 5) * 5);
    const y = Math.max(NODE_H / 2 + 4, Math.round((p.y - S.drag.dy) / 5) * 5);
    if (x !== n.x || y !== n.y) {
      n.x = x; n.y = y;
      S.drag.moved = true;
      dirty();
      renderCanvas();
    }
  }

  function onPointerUp(e) {
    if (!S.drag) return;
    const d = S.drag;
    S.drag = null;
    try { svg.releasePointerCapture(e.pointerId); } catch (err) { /* já liberado */ }
    if (!d.moved) S.sel = { type: "node", host: d.host };
    renderCanvas(); renderSide();
  }

  function deleteSelection() {
    const s = site();
    if (!s || !S.sel || S.mode !== "edit") return;
    if (S.sel.type === "node") {
      if (isRoot(s, S.sel.host)) return;
      if (!confirm(`Remover ${S.sel.host} e os links dele?`)) return;
      removeNode(s, S.sel.host);
    }
    else if (S.sel.type === "link") {
      s.links.splice(S.sel.idx, 1);
    }
    S.sel = null;
    dirty();
    renderCanvas(); renderSide();
  }

  // ---------- ações ----------

  function addSite() {
    const name = (prompt("Nome da localidade (ex: UHCC):") || "").trim();
    if (!name) return;
    if (S.sites.some((s) => lc(s.name) === lc(name))) { alert("Já existe uma localidade com esse nome."); return; }
    S.sites.push(newSite(name));
    S.cur = S.sites.length - 1;
    S.sel = null; S.linkFrom = null; S.sim.down.clear(); S.sim.siteDown = false;
    dirty();
    renderAll();
  }

  function removeSite() {
    const s = site();
    if (!s || !confirm(`Remover a localidade ${s.name} inteira?`)) return;
    S.sites.splice(S.cur, 1);
    S.cur = Math.max(0, S.cur - 1);
    S.sel = null; S.linkFrom = null;
    dirty();
    renderAll();
  }

  function addNode() {
    const s = site();
    if (!s) return;
    const inp = document.getElementById("gd-topo-newhost");
    const host = inp.value.trim();
    if (!host) return;
    if (nodeOf(s, host)) { S.sel = { type: "node", host: nodeOf(s, host).host }; renderAll(); return; }
    const other = siteOfHost(host);
    if (other >= 0 && other !== S.cur) { alert(`${host} já está na localidade ${S.sites[other].name}.`); return; }
    if (lc(host) === lc(s.status.host)) { alert("Esse é o host de status: ele é avaliado à parte e não entra na topologia."); return; }
    const p = freeSpot(s);
    s.nodes.push({ host, rack: document.getElementById("gd-topo-newrack").value.trim(), x: p.x, y: p.y });
    inp.value = "";
    S.sel = { type: "node", host };
    dirty();
    renderAll();
  }

  function importGroup() {
    const s = site();
    if (!s) { alert("Crie ou escolha uma localidade antes."); return; }
    const group = document.getElementById("gd-topo-group").value.trim();
    if (!group) return;

    fetch("zabbix.php?action=godesk.dependencies.v2.data", {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: new URLSearchParams({ op: "group", group }),
    })
      .then((r) => r.json())
      .then((data) => {
        if (data.error) { alert(data.error); return; }
        const fresh = s.nodes.length <= 1;
        let added = 0;
        const skipped = [];

        if (!s.status.host && (data.root_hosts || []).length === 1) {
          s.status.host = data.root_hosts[0];
        }

        const order = (name) => { const i = LAYER_ORDER.indexOf(lc(name)); return i < 0 ? LAYER_ORDER.length : i; };
        const subs = (data.subgroups || []).slice().sort((a, b) => order(a.name) - order(b.name) || a.name.localeCompare(b.name));
        const routers = [];

        subs.forEach((sg) => {
          sg.hosts.forEach((h) => {
            // links de operadora ficam pra fase 2
            if (/^link\b/i.test(h)) { skipped.push(h); return; }
            if (lc(h) === lc(s.status.host) || nodeOf(s, h)) return;
            const other = siteOfHost(h);
            if (other >= 0 && other !== S.cur) { skipped.push(h); return; }
            s.nodes.push({ host: h, rack: "", x: 0, y: 0 });
            if (lc(sg.name) === "router") routers.push(h);
            added++;
          });
        });

        // os SDs (grupo Router) são a entrada: liga o root neles se ainda não houver link do root
        if (!(adjacency(s)[lc(s.root)] || new Set()).size) {
          routers.forEach((h) => s.links.push([s.root, h]));
        }

        if (fresh) {
          // posiciona por camada do grupo (Router, Switch/Core, Firewall, Switch, RTAC...)
          const rowsY = {};
          let y = 50;
          const root = nodeOf(s, s.root);
          root.y = y;
          subs.forEach((sg) => { y += 100; rowsY[lc(sg.name)] = y; });
          subs.forEach((sg) => {
            const row = s.nodes.filter((n) => n.x === 0 && n.y === 0 && sg.hosts.some((h) => lc(h) === lc(n.host)));
            const total = row.reduce((a, n) => a + nodeWidth(n.host) + 24, -24);
            let x = Math.max(20, (Math.max(900, total + 80) - total) / 2);
            row.forEach((n) => { n.x = Math.round(x + nodeWidth(n.host) / 2); n.y = rowsY[lc(sg.name)]; x += nodeWidth(n.host) + 24; });
          });
          const W = Math.max(...s.nodes.map((n) => n.x));
          root.x = Math.round(W / 2);
        }
        else {
          autoLayout(s, false);
        }

        dirty();
        renderAll();
        alert(`${added} hosts adicionados.` + (skipped.length ? `\nIgnorados (links de operadora ou já em outra localidade): ${skipped.join(", ")}` : "") +
          "\n\nAgora desenhe os links (modo 🔗 Ligar) e marque os racks.");
      })
      .catch((err) => alert("Falha ao consultar o grupo: " + err));
  }

  function loadLiveState() {
    const s = site();
    if (!s) return;
    const hosts = s.nodes.filter((n) => !isRoot(s, n.host)).map((n) => n.host);
    if (s.status.host) hosts.push(s.status.host);
    const body = new URLSearchParams({ op: "state" });
    hosts.forEach((h) => body.append("hosts[]", h));

    fetch("zabbix.php?action=godesk.dependencies.v2.data", {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body,
    })
      .then((r) => r.json())
      .then((data) => {
        if (data.error) { alert(data.error); return; }
        const problems = {};
        Object.keys(data.problems || {}).forEach((h) => { problems[lc(h)] = data.problems[h]; });
        S.sim.down.clear();
        s.nodes.forEach((n) => {
          if ((problems[lc(n.host)] || []).some((p) => matchEvent(s.down_event, p))) S.sim.down.add(lc(n.host));
        });
        S.sim.siteDown = !!s.status.host && (problems[lc(s.status.host)] || []).some((p) => matchEvent(s.status.event, p));
        renderAll();
      })
      .catch((err) => alert("Falha ao consultar o Zabbix: " + err));
  }

  function setSiteField(field, value) {
    const s = site();
    if (!s) return;
    if (field === "root") {
      const v = value.trim();
      if (!v || nodeOf(s, v) && !isRoot(s, v)) return;
      renameHost(s, s.root, v);
      s.root = v;
    }
    else if (field === "status.host") s.status.host = value.trim();
    else if (field === "status.event") s.status.event = value;
    else s[field] = value;
    dirty();
    if (field === "name") renderSiteSelect();
    renderModes();
    renderCanvas();
    renderSide();
  }

  function cleanSites() {
    return S.sites.map((s) => ({
      name: s.name.trim(),
      root: s.root.trim(),
      down_event: s.down_event.trim(),
      status: { host: s.status.host.trim(), event: s.status.event.trim() },
      nodes: s.nodes.map((n) => ({ host: n.host, rack: n.rack, x: Math.round(n.x), y: Math.round(n.y) })),
      links: s.links.map((l) => [l[0], l[1]]),
    }));
  }

  function exportYaml() {
    const text = document.getElementById("gd-topo-export-yaml").value;
    if (!text.trim()) { alert("Nenhuma arquitetura salva para exportar."); return; }
    const blob = new Blob([text], { type: "text/yaml" });
    const a = document.createElement("a");
    a.href = URL.createObjectURL(blob);
    a.download = "godesk-dependencies-v2.yaml";
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(a.href);
  }

  function importYaml(input) {
    const file = input.files && input.files[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = () => {
      input.value = "";
      if (!confirm(`Importar "${file.name}"? Isso SUBSTITUI toda a arquitetura atual (um backup do arquivo atual é mantido no servidor).`)) return;
      const form = document.getElementById("gd-topo-import-form");
      form.querySelector('textarea[name="import_yaml"]').value = String(reader.result || "");
      S.dirty = false;
      form.submit();
    };
    reader.readAsText(file);
  }

  // ---------- init ----------

  function init() {
    const box = document.getElementById("gd-topo");
    const dataEl = document.getElementById("gd-topo-data");
    if (!box || !dataEl) return;

    let data = {};
    try { data = JSON.parse(dataEl.textContent || "{}"); } catch (e) { data = {}; }

    S = {
      defaults: Object.assign({ root: "ZABBIX", down_event: "*Unavailable by ICMP ping*" }, data.defaults || {}),
      sites: [],
      cur: 0,
      mode: "edit",
      sel: null,
      linkFrom: null,
      drag: null,
      dirty: false,
      sim: { down: new Set(), siteDown: false },
    };
    S.sites = (data.sites || []).map(prepSite);

    const last = storageGet("gd-topo-site");
    const idx = S.sites.findIndex((s) => s.name === last);
    if (idx >= 0) S.cur = idx;

    svg = document.getElementById("gd-topo-svg");
    svg.addEventListener("pointerdown", onPointerDown);
    svg.addEventListener("pointermove", onPointerMove);
    svg.addEventListener("pointerup", onPointerUp);
    svg.addEventListener("pointercancel", onPointerUp);

    box.addEventListener("click", (e) => {
      const act = e.target.closest("[data-gd-topo]");
      if (act) {
        const a = act.dataset.gdTopo;
        if (a === "site-add") addSite();
        else if (a === "site-remove") removeSite();
        else if (a === "node-add") addNode();
        else if (a === "group-import") importGroup();
        else if (a === "layout") { const s = site(); if (s) { autoLayout(s, true); dirty(); renderCanvas(); } }
        else if (a === "node-remove") deleteSelection();
        else if (a === "sim-clear") { S.sim.down.clear(); S.sim.siteDown = false; renderAll(); }
        else if (a === "sim-live") loadLiveState();
        return;
      }
      const mode = e.target.closest("[data-gd-topo-mode]");
      if (mode) {
        S.mode = mode.dataset.gdTopoMode;
        S.linkFrom = null;
        if (S.mode !== "edit") S.sel = null;
        renderAll();
        return;
      }
      const unlink = e.target.closest("[data-gd-topo-unlink]");
      if (unlink) {
        const s = site();
        s.links.splice(+unlink.dataset.gdTopoUnlink, 1);
        if (S.sel && S.sel.type === "link") S.sel = null;
        dirty();
        renderCanvas(); renderSide();
      }
    });

    box.addEventListener("input", (e) => {
      const f = e.target.dataset && e.target.dataset.gdSiteField;
      if (f && f !== "root") setSiteField(f, e.target.value);
      if (e.target.id === "gd-topo-sel-rack") {
        const n = S.sel && nodeOf(site(), S.sel.host);
        if (n) { n.rack = e.target.value.trim(); dirty(); renderCanvas(); }
      }
    });

    box.addEventListener("change", (e) => {
      if (e.target.id === "gd-topo-site") {
        S.cur = +e.target.value || 0;
        S.sel = null; S.linkFrom = null; S.sim.down.clear(); S.sim.siteDown = false;
        storageSet("gd-topo-site", (site() || {}).name || "");
        renderAll();
        return;
      }
      if (e.target.dataset && e.target.dataset.gdSiteField === "root") {
        setSiteField("root", e.target.value);
        renderSiteProps();
        return;
      }
      if (e.target.id === "gd-topo-sim-sitedown") {
        S.sim.siteDown = e.target.checked;
        renderCanvas(); renderSide();
        return;
      }
      if (e.target.id === "gd-topo-sel-host") {
        const s = site();
        const v = e.target.value.trim();
        if (!v || !S.sel) return;
        if (nodeOf(s, v) && lc(v) !== lc(S.sel.host)) { alert("Já existe um host com esse nome nesta localidade."); e.target.value = S.sel.host; return; }
        const other = siteOfHost(v);
        if (other >= 0 && other !== S.cur) { alert(`${v} já está na localidade ${S.sites[other].name}.`); e.target.value = S.sel.host; return; }
        renameHost(s, S.sel.host, v);
        S.sel.host = v;
        dirty();
        renderCanvas(); renderSide();
      }
    });

    document.getElementById("gd-topo-newhost").addEventListener("keydown", (e) => {
      if (e.key === "Enter") { e.preventDefault(); addNode(); }
    });

    document.addEventListener("keydown", (e) => {
      if (e.key !== "Delete" && e.key !== "Backspace") return;
      const t = document.activeElement;
      if (t && /^(INPUT|TEXTAREA|SELECT)$/.test(t.tagName)) return;
      if (S.sel) { e.preventDefault(); deleteSelection(); }
    });

    document.querySelector("[data-gd-topo-export]").addEventListener("click", exportYaml);
    document.querySelector("[data-gd-topo-import]").addEventListener("click", () => document.getElementById("gd-topo-import-file").click());
    document.getElementById("gd-topo-import-file").addEventListener("change", (e) => importYaml(e.target));

    document.getElementById("gd-topo-form").addEventListener("submit", () => {
      document.getElementById("gd-topo-json").value = JSON.stringify({ sites: cleanSites() });
      storageSet("gd-topo-site", (site() || {}).name || "");
      S.dirty = false;
    });

    window.addEventListener("beforeunload", (e) => {
      if (S.dirty) { e.preventDefault(); e.returnValue = ""; }
    });
    window.addEventListener("resize", () => renderCanvas());

    renderAll();
  }

  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", init);
  else init();
})();
