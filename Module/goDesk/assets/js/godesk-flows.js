/* =========================================================
   goDesk — editor visual dos fluxos de decisão das rules
   Página: godesk.flows.edit (só roda se #gd-flow existir)

   Estado em JSON (mesmo schema do godesk-flows.yaml, ver
   internal/config/flows.go), desenhado em SVG:
     - arraste os blocos; arraste de uma saída (bolinha embaixo) até outro
       bloco pra ligar; clique numa ligação e Delete remove
     - painel lateral edita o bloco selecionado
     - 🧪 Testar roda o fluxo com dados de um evento (port do
       internal/flow/engine.go) e destaca o caminho
   Salvar manda tudo num campo só (flows_json); o PHP valida com o goDesk
   (godesk-client flows-check) e grava.
========================================================= */
(function () {
  "use strict";

  const SVGNS = "http://www.w3.org/2000/svg";
  const NODE_W = 220;
  const HEAD_H = 26;
  const LINE_H = 16;
  const PORT_ZONE = 30;
  const MAX_STEPS = 200;

  const TYPES = {
    start: { icon: "▶", name: "Início" },
    if: { icon: "◇", name: "Condição" },
    switch: { icon: "⑂", name: "Switch" },
    set: { icon: "✎", name: "Definir campos" },
    end: { icon: "■", name: "Fim" },
  };

  const OPS = [
    ["eq", "igual a"], ["neq", "diferente de"],
    ["contains", "contém"], ["not_contains", "não contém"],
    ["matches", "corresponde (*)"], ["not_matches", "não corresponde (*)"],
    ["in", "está na lista"], ["not_in", "não está na lista"],
    ["exists", "preenchido"], ["not_exists", "vazio"],
    ["gt", ">"], ["gte", "≥"], ["lt", "<"], ["lte", "≤"],
    ["is_true", "é verdadeiro"], ["is_false", "é falso"],
  ];
  const OP_LABEL = Object.fromEntries(OPS);
  const NOVALUE_OPS = new Set(["exists", "not_exists", "is_true", "is_false"]);

  // mesma lista do config.FlowSettableFields
  const SETTABLE = [
    ["instrucao", "Instrução (mensagem pro atendimento)"],
    ["urgency", "Urgência"], ["impact", "Impacto"], ["priority", "Prioridade"],
    ["oper_group", "Grupo operador (ID)"], ["operator", "Operador (ID)"],
    ["contract", "Contrato"], ["main_caller", "Solicitante principal"], ["secundary_caller", "Solicitante secundário"],
    ["sla", "SLA (ID)"], ["category", "Categoria"], ["sub_category", "Subcategoria"], ["call_type", "Tipo de chamado"],
    ["equipe", "Equipe"], ["cliente", "Cliente (exibição)"],
    ["send_teams", "Enviar Teams"], ["teams_webhook", "Webhook do Teams"],
    ["send_email", "Enviar e-mail"], ["email_to", "E-mail para"], ["email_cc", "E-mail cópia"], ["once_per_day", "E-mail 1x por dia"],
    ["send_more_info", "Enviar mais informações"], ["more_info_text", "Texto de mais informações"],
    ["autoclose", "Autoclose"], ["dependency", "Dependência"], ["dependency_version", "Versão da dependência (v1/v2/shadow)"],
    ["custom_status", "Custom status"], ["status_open", "Status abertura (ID)"], ["status_update", "Status atualização (ID)"],
  ];
  const SETTABLE_LABEL = Object.fromEntries(SETTABLE);
  const BOOL_FIELDS = new Set(["autoclose", "dependency", "custom_status", "send_teams", "send_email", "once_per_day", "send_more_info"]);
  const LONG_FIELDS = new Set(["instrucao", "more_info_text"]);

  // mesmos campos do eventResolver (internal/app/flow.go) + os do motor
  const FIELDS = [
    ["host", "host (nome técnico)"], ["host.name", "host (nome visível)"], ["host.group", "grupos do host"],
    ["host.tag.", "tag do host — host.tag.<nome>"], ["event.tag.", "tag do evento — event.tag.<nome>"],
    ["trigger", "nome do trigger/problema"], ["severity", "severidade (nome)"], ["severity.level", "severidade 0 a 5"],
    ["event.kind", "problem / recovery"], ["status", "status (PROBLEM/RESOLVED)"], ["value", "valor do item"],
    ["rule", "rule"], ["cliente", "cliente"], ["equipe", "equipe"],
    ["ticket.exists", "chamado já existe (true/false)"], ["ticket.status", "status do chamado"],
    ["now.weekday", "dia da semana (dom..sab)"], ["now.hour", "hora (00-23)"], ["now.time", "hora HH:MM"], ["now.date", "data AAAA-MM-DD"],
    ["set.", "campo já definido no fluxo — set.<campo>"],
  ];
  const VARS = ["host", "host.name", "trigger", "severity", "value", "host.tag.", "event.tag.", "cliente", "now.time"];

  const DAYS = ["dom", "seg", "ter", "qua", "qui", "sex", "sab"];
  const WEEKDAY = { dom: 0, seg: 1, ter: 2, qua: 3, qui: 4, sex: 5, sab: 6, "sáb": 6, sun: 0, mon: 1, tue: 2, wed: 3, thu: 4, fri: 5, sat: 6 };

  let S = null;
  let svg = null;

  const lc = (s) => String(s == null ? "" : s).trim().toLowerCase();
  const $ = (id) => document.getElementById(id);

  function escapeHtml(s) {
    return String(s == null ? "" : s)
      .replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;").replace(/'/g, "&#39;");
  }
  function trunc(s, n) { s = String(s == null ? "" : s); return s.length > n ? s.slice(0, n - 1) + "…" : s; }
  function storageGet(k) { try { return window.sessionStorage.getItem(k); } catch (e) { return null; } }
  function storageSet(k, v) { try { window.sessionStorage.setItem(k, v); } catch (e) { /* sem storage */ } }

  // =========================================================
  // motor — port do internal/flow/engine.go (manter igual!)
  // =========================================================

  function parseBool(v) {
    switch (lc(v)) {
      case "true": case "sim": case "1": case "yes": case "on": return [true, true];
      case "false": case "não": case "nao": case "0": case "no": case "off": return [false, true];
    }
    return [false, false];
  }

  function wildcard(pattern, s) {
    pattern = lc(pattern); s = lc(s);
    if (pattern.indexOf("*") < 0) return pattern === s;
    const parts = pattern.split("*");
    if (!s.startsWith(parts[0])) return false;
    s = s.slice(parts[0].length);
    const last = parts[parts.length - 1];
    for (const p of parts.slice(1, -1)) {
      const i = s.indexOf(p);
      if (i < 0) return false;
      s = s.slice(i + p.length);
    }
    return s.endsWith(last);
  }

  function nonEmpty(vals) { return (vals || []).filter((v) => String(v).trim() !== ""); }
  function showValues(vals) { const v = nonEmpty(vals); return v.length ? v.join(", ") : "vazio"; }
  function splitList(s) { return String(s).split(",").map((p) => p.trim()).filter(Boolean); }
  function num(v) {
    const t = String(v == null ? "" : v).trim().replace(/,/g, ".");
    if (t === "") return NaN;
    return Number(t);
  }

  function evalCondition(vals, op, want) {
    vals = vals || [];
    const any = (fn) => vals.some(fn);
    switch (op) {
      case "": case "eq": return any((v) => String(v).toLowerCase() === String(want).toLowerCase());
      case "neq": return !evalCondition(vals, "eq", want);
      case "contains": return any((v) => String(v).toLowerCase().includes(String(want).toLowerCase()));
      case "not_contains": return !evalCondition(vals, "contains", want);
      case "matches": return any((v) => wildcard(want, v));
      case "not_matches": return !evalCondition(vals, "matches", want);
      case "in": { const list = splitList(want); return any((v) => list.some((w) => wildcard(w, v))); }
      case "not_in": return !evalCondition(vals, "in", want);
      case "exists": return nonEmpty(vals).length > 0;
      case "not_exists": return nonEmpty(vals).length === 0;
      case "gt": case "gte": case "lt": case "lte": {
        const w = num(want);
        if (isNaN(w)) return false;
        return any((v) => {
          const f = num(v);
          if (isNaN(f)) return false;
          return op === "gt" ? f > w : op === "gte" ? f >= w : op === "lt" ? f < w : f <= w;
        });
      }
      case "is_true": return any((v) => { const [b, ok] = parseBool(v); return ok && b; });
      case "is_false": return !evalCondition(vals, "is_true", want);
    }
    return false;
  }

  function findCalendar(cals, name) {
    if (cals[name]) return cals[name];
    const k = Object.keys(cals).find((x) => lc(x) === lc(name));
    return k ? cals[k] : null;
  }

  function pad(n) { return String(n).padStart(2, "0"); }
  function fmtDate(d) { return d.getFullYear() + "-" + pad(d.getMonth() + 1) + "-" + pad(d.getDate()); }
  function fmtTime(d) { return pad(d.getHours()) + ":" + pad(d.getMinutes()); }

  function inCalendar(c, now) {
    const date = fmtDate(now);
    if ((c.holidays || []).some((h) => String(h).trim() === date)) return false;
    if (!(c.days || []).some((d) => WEEKDAY[lc(d)] === now.getDay())) return false;
    const hm = fmtTime(now);
    return hm >= c.start && hm < c.end;
  }

  function runFlow(data, fl, resolve, now) {
    const r = { flow: fl.name, action: "open", comment: "", set: [], trace: [], error: "" };
    const valueOf = (field) => { for (let i = r.set.length - 1; i >= 0; i--) if (r.set[i].field === field) return r.set[i].value; return null; };

    const values = (field) => {
      field = String(field || "").trim();
      const lower = field.toLowerCase();
      if (lower.startsWith("calendar.")) {
        const c = findCalendar(data.calendars || {}, field.slice(9));
        return c ? [String(inCalendar(c, now))] : [];
      }
      if (lower === "now.weekday") return [DAYS[now.getDay()]];
      if (lower === "now.hour") return [pad(now.getHours())];
      if (lower === "now.time") return [fmtTime(now)];
      if (lower === "now.date") return [fmtDate(now)];
      if (lower.startsWith("set.")) { const v = valueOf(lower.slice(4)); return v == null ? [] : [v]; }
      return resolve(field) || [];
    };
    const render = (tpl) => {
      tpl = String(tpl == null ? "" : tpl);
      if (tpl.indexOf("{") < 0) return tpl;
      return tpl.replace(/\{([^{}]+)\}/g, (m, f) => nonEmpty(values(f)).join(", "));
    };

    let cur = fl.nodes.find((n) => n.type === "start");
    if (!cur) { r.error = "fluxo sem bloco de início"; return r; }

    for (let i = 0; i < MAX_STEPS; i++) {
      const step = { node: cur.id, type: cur.type, label: cur.label || "", port: "", detail: "" };
      let port = "";

      if (cur.type === "start") {
        port = "next";
      }
      else if (cur.type === "if") {
        const anyMode = cur.match === "any";
        let result = !anyMode;
        const parts = [];
        (cur.conditions || []).forEach((c) => {
          const vals = values(c.field);
          const want = render(c.value);
          const ok = evalCondition(vals, c.op || "eq", want);
          parts.push(`${ok ? "✓" : "✗"} ${c.field} ${c.op || "eq"}${want !== "" ? " " + want : ""} (${showValues(vals)})`);
          if (anyMode && ok) result = true;
          if (!anyMode && !ok) result = false;
        });
        step.detail = parts.join(anyMode ? " OU " : " E ");
        port = result ? "true" : "false";
      }
      else if (cur.type === "switch") {
        const vals = values(cur.field);
        port = "default";
        for (const c of cur.cases || []) {
          if (evalCondition(vals, "in", c.value)) { port = c.value; break; }
        }
        step.detail = `${cur.field} = ${showValues(vals)}`;
      }
      else if (cur.type === "set") {
        const done = [];
        (cur.set || []).forEach((a) => {
          const v = render(a.value);
          r.set.push({ field: a.field, value: v });
          done.push(a.field + "=" + v);
        });
        step.detail = done.join("; ");
        port = "next";
      }
      else if (cur.type === "end") {
        r.action = cur.action || "open";
        r.comment = render(cur.comment || "");
        step.detail = r.action;
        r.trace.push(step);
        return r;
      }
      else {
        r.error = `bloco ${cur.id}: tipo ${cur.type} desconhecido`;
        return r;
      }

      step.port = port;
      r.trace.push(step);
      const link = linkFrom(fl, cur.id, port);
      if (!link) return r;
      const next = fl.nodes.find((n) => n.id === link.to);
      if (!next) { r.error = `bloco ${cur.id} liga em ${link.to}, que não existe`; return r; }
      cur = next;
    }
    r.error = `passou de ${MAX_STEPS} passos (ciclo?)`;
    return r;
  }

  function linkPort(l) { return l.port ? l.port : "next"; }
  function linkFrom(fl, id, port) { return fl.links.find((l) => l.from === id && lc(linkPort(l)) === lc(port)) || null; }

  // =========================================================
  // modelo
  // =========================================================

  function flow() { return S.data.flows[S.cur] || null; }
  function nodeOf(fl, id) { return fl.nodes.find((n) => n.id === id) || null; }
  function dirty() { S.dirty = true; }

  function ports(n) {
    switch (n.type) {
      case "start": case "set": return [{ port: "next", label: "" }];
      case "if": return [{ port: "true", label: "sim" }, { port: "false", label: "não" }];
      case "switch": return (n.cases || []).map((c) => ({ port: c.value, label: c.value })).concat([{ port: "default", label: "outro" }]);
      default: return [];
    }
  }

  function newId(fl, type) {
    let i = 1;
    while (fl.nodes.some((n) => n.id === type + i)) i++;
    return type + i;
  }

  function newNode(fl, type, x, y) {
    const n = { id: newId(fl, type), type, label: "", x: Math.round(x), y: Math.round(y) };
    if (type === "if") { n.match = "all"; n.conditions = [{ field: "severity.level", op: "gte", value: "4" }]; }
    if (type === "switch") { n.field = "host.tag.Criticidade"; n.cases = [{ value: "Critico" }]; }
    if (type === "set") { n.set = [{ field: "instrucao", value: "" }]; }
    if (type === "end") { n.action = "open"; n.comment = ""; }
    return n;
  }

  function newFlow(name) {
    return { name, description: "", nodes: [{ id: "inicio", type: "start", label: "", x: 420, y: 30 }], links: [] };
  }

  function prepData(raw) {
    const d = { calendars: {}, flows: [] };
    Object.keys(raw.calendars || {}).forEach((k) => {
      const c = raw.calendars[k] || {};
      d.calendars[k] = { days: (c.days || []).map(String), start: String(c.start || "08:00"), end: String(c.end || "18:00"), holidays: (c.holidays || []).map(String) };
    });
    (raw.flows || []).forEach((f) => {
      const fl = { name: String(f.name || ""), description: String(f.description || ""), nodes: [], links: [] };
      (f.nodes || []).forEach((n) => {
        const node = { id: String(n.id || ""), type: String(n.type || ""), label: String(n.label || ""), x: +n.x || 0, y: +n.y || 0 };
        if (node.type === "if") { node.match = n.match === "any" ? "any" : "all"; node.conditions = (n.conditions || []).map((c) => ({ field: String(c.field || ""), op: String(c.op || "eq"), value: String(c.value == null ? "" : c.value) })); }
        if (node.type === "switch") { node.field = String(n.field || ""); node.cases = (n.cases || []).map((c) => ({ value: String(c.value == null ? "" : c.value) })); }
        if (node.type === "set") { node.set = (n.set || []).map((a) => ({ field: String(a.field || ""), value: String(a.value == null ? "" : a.value) })); }
        if (node.type === "end") { node.action = n.action === "skip" ? "skip" : "open"; node.comment = String(n.comment || ""); }
        fl.nodes.push(node);
      });
      fl.links = (f.links || []).map((l) => {
        const o = { from: String(l.from || ""), to: String(l.to || "") };
        if (l.port && l.port !== "next") o.port = String(l.port);
        return o;
      });
      if (!fl.nodes.some((n) => n.type === "start")) fl.nodes.unshift({ id: "inicio", type: "start", label: "", x: 420, y: 30 });
      if (fl.nodes.every((n) => n.x === 0 && n.y === 0)) autoLayout(fl);
      d.flows.push(fl);
    });
    return d;
  }

  function cleanData() {
    return {
      calendars: S.data.calendars,
      flows: S.data.flows.map((f) => ({
        name: f.name.trim(),
        description: f.description.trim(),
        nodes: f.nodes.map((n) => {
          const o = { id: n.id, type: n.type, label: n.label.trim(), x: Math.round(n.x), y: Math.round(n.y) };
          if (n.type === "if") { o.match = n.match; o.conditions = n.conditions.map((c) => ({ field: c.field.trim(), op: c.op, value: NOVALUE_OPS.has(c.op) ? "" : c.value })); }
          if (n.type === "switch") { o.field = n.field.trim(); o.cases = n.cases.map((c) => ({ value: c.value.trim() })); }
          if (n.type === "set") { o.set = n.set.map((a) => ({ field: a.field, value: a.value })); }
          if (n.type === "end") { o.action = n.action; o.comment = n.action === "skip" ? n.comment : ""; }
          return o;
        }),
        links: f.links.map((l) => (l.port ? { from: l.from, port: l.port, to: l.to } : { from: l.from, to: l.to })),
      })),
    };
  }

  // validação rápida no navegador (o goDesk valida de novo ao salvar)
  function validate() {
    const errs = [];
    const names = new Set();
    Object.keys(S.data.calendars).forEach((k) => {
      const c = S.data.calendars[k];
      if (!c.days.length) errs.push(`Calendário ${k}: marque pelo menos um dia.`);
      if (!/^([01]\d|2[0-3]):[0-5]\d$/.test(c.start) || !/^([01]\d|2[0-3]):[0-5]\d$/.test(c.end) || c.start >= c.end) errs.push(`Calendário ${k}: início/fim inválidos (HH:MM, início antes do fim).`);
      c.holidays.forEach((h) => { if (!/^\d{4}-\d{2}-\d{2}$/.test(h)) errs.push(`Calendário ${k}: feriado ${h} no formato AAAA-MM-DD.`); });
    });
    const calRef = (field) => {
      if (lc(field).startsWith("calendar.") && !findCalendar(S.data.calendars, field.trim().slice(9))) return `calendário ${field.trim().slice(9)} não cadastrado`;
      return "";
    };
    const tplRefs = (s) => (String(s).match(/\{([^{}]+)\}/g) || []).map((m) => calRef(m.slice(1, -1))).filter(Boolean);

    S.data.flows.forEach((f, fi) => {
      const fname = f.name.trim() || `#${fi + 1}`;
      if (!f.name.trim()) errs.push(`Fluxo #${fi + 1}: informe o nome.`);
      else if (names.has(lc(f.name))) errs.push(`Fluxo ${f.name} repetido.`);
      names.add(lc(f.name));
      f.nodes.forEach((n) => {
        const who = `Fluxo ${fname}, bloco ${n.label || n.id}`;
        if (n.type === "if") {
          if (!n.conditions.length) errs.push(`${who}: adicione pelo menos uma regra.`);
          n.conditions.forEach((c, i) => {
            if (!c.field.trim()) errs.push(`${who}: regra #${i + 1} sem campo.`);
            const e = calRef(c.field); if (e) errs.push(`${who}: ${e}.`);
            tplRefs(c.value).forEach((e2) => errs.push(`${who}: ${e2}.`));
          });
        }
        if (n.type === "switch") {
          if (!n.field.trim()) errs.push(`${who}: informe o campo.`);
          const e = calRef(n.field); if (e) errs.push(`${who}: ${e}.`);
          if (!n.cases.length) errs.push(`${who}: adicione pelo menos um caso.`);
          const seen = new Set(["default"]);
          n.cases.forEach((c, i) => {
            if (!c.value.trim()) errs.push(`${who}: caso #${i + 1} vazio.`);
            else if (seen.has(lc(c.value))) errs.push(`${who}: caso ${c.value} repetido.`);
            seen.add(lc(c.value));
          });
        }
        if (n.type === "set") {
          if (!n.set.length) errs.push(`${who}: adicione pelo menos um campo.`);
          n.set.forEach((a) => {
            if (BOOL_FIELDS.has(a.field) && !parseBool(a.value)[1]) errs.push(`${who}: ${a.field} precisa ser sim/não.`);
            tplRefs(a.value).forEach((e2) => errs.push(`${who}: ${e2}.`));
          });
        }
        if (n.type === "end") tplRefs(n.comment).forEach((e2) => errs.push(`${who}: ${e2}.`));
      });
    });
    return errs;
  }

  // =========================================================
  // layout
  // =========================================================

  function nodeWidth(n) { return n.type === "switch" ? Math.max(NODE_W, ports(n).length * 76) : NODE_W; }

  function summary(fl, n) {
    const w = Math.floor(nodeWidth(n) / 6.6);
    switch (n.type) {
      case "start": return ["Alerta recebido"];
      case "if": {
        const join = n.match === "any" ? "OU " : "E ";
        const lines = n.conditions.slice(0, 3).map((c, i) => trunc((i ? join : "") + `${c.field} ${OP_LABEL[c.op] || c.op}${NOVALUE_OPS.has(c.op) ? "" : " " + c.value}`, w));
        if (n.conditions.length > 3) lines.push(`+${n.conditions.length - 3} regra(s)`);
        return lines.length ? lines : ["(sem regras)"];
      }
      case "switch": return [trunc("valor de " + (n.field || "?"), w)];
      case "set": {
        const lines = n.set.slice(0, 3).map((a) => trunc(`${a.field} = ${a.value}`, w));
        if (n.set.length > 3) lines.push(`+${n.set.length - 3} campo(s)`);
        return lines.length ? lines : ["(nenhum campo)"];
      }
      case "end": return [n.action === "skip" ? "Não abrir chamado" : "Abrir chamado"].concat(n.action === "skip" && n.comment ? [trunc(n.comment, w)] : []);
    }
    return [];
  }

  function nodeHeight(fl, n) { return HEAD_H + summary(fl, n).length * LINE_H + 10 + (ports(n).length ? PORT_ZONE - 10 : 0); }

  function portPos(fl, n, port) {
    const ps = ports(n);
    const w = nodeWidth(n);
    const i = Math.max(0, ps.findIndex((p) => lc(p.port) === lc(port)));
    return { x: n.x - w / 2 + ((i + 1) * w) / (ps.length + 1), y: n.y + nodeHeight(fl, n) };
  }

  function autoLayout(fl) {
    const level = {};
    const start = fl.nodes.find((n) => n.type === "start");
    if (start) {
      level[start.id] = 0;
      const q = [start.id];
      while (q.length) {
        const cur = q.shift();
        const n = nodeOf(fl, cur);
        // ordem das portas define a ordem na linha
        ports(n).forEach((p) => {
          const l = linkFrom(fl, cur, p.port);
          if (l && level[l.to] === undefined) { level[l.to] = level[cur] + 1; q.push(l.to); }
        });
      }
    }
    let maxL = 0;
    Object.values(level).forEach((l) => { maxL = Math.max(maxL, l); });
    const rows = {};
    const order = [];
    // BFS de novo pra pegar a ordem de descoberta
    if (start) {
      const seen = new Set([start.id]);
      const q = [start.id];
      while (q.length) {
        const cur = q.shift();
        order.push(cur);
        ports(nodeOf(fl, cur)).forEach((p) => {
          const l = linkFrom(fl, cur, p.port);
          if (l && !seen.has(l.to)) { seen.add(l.to); q.push(l.to); }
        });
      }
    }
    fl.nodes.forEach((n) => { if (!order.includes(n.id)) order.push(n.id); });
    order.forEach((id) => {
      const l = level[id] === undefined ? maxL + 1 : level[id];
      (rows[l] = rows[l] || []).push(nodeOf(fl, id));
    });

    const W = Math.max(900, ...Object.values(rows).map((r) => r.reduce((a, n) => a + nodeWidth(n) + 40, 0) + 80));
    let y = 30;
    Object.keys(rows).map(Number).sort((a, b) => a - b).forEach((l) => {
      const row = rows[l];
      const total = row.reduce((a, n) => a + nodeWidth(n) + 40, -40);
      let x = (W - total) / 2;
      let h = 0;
      row.forEach((n) => {
        n.x = Math.round(x + nodeWidth(n) / 2);
        n.y = y;
        x += nodeWidth(n) + 40;
        h = Math.max(h, nodeHeight(fl, n));
      });
      y += h + 70;
    });
  }

  function freeSpot(fl) {
    for (let y = 30; y < 6000; y += 120) {
      for (let x = 140; x < 1400; x += 260) {
        if (!fl.nodes.some((n) => Math.abs(n.x - x) < 230 && Math.abs(n.y - y) < 100)) return { x, y };
      }
    }
    return { x: 140, y: 30 };
  }

  // =========================================================
  // render
  // =========================================================

  function el(tag, attrs, parent) {
    const e = document.createElementNS(SVGNS, tag);
    Object.keys(attrs || {}).forEach((k) => e.setAttribute(k, attrs[k]));
    if (parent) parent.appendChild(e);
    return e;
  }

  function renderAll() {
    renderFlowSelect();
    renderRules();
    renderCalendars();
    renderFieldList();
    renderCanvas();
    renderSide();
    renderTest();
  }

  function renderFlowSelect() {
    const sel = $("gd-flow-select");
    sel.innerHTML = S.data.flows.length
      ? S.data.flows.map((f, i) => `<option value="${i}"${i === S.cur ? " selected" : ""}>${escapeHtml(f.name || "(sem nome)")}</option>`).join("")
      : '<option value="">— nenhum —</option>';
  }

  function renderRules() {
    const box = $("gd-flow-rules");
    const fl = flow();
    const list = fl ? S.rulesByFlow[lc(fl.name)] || [] : [];
    box.innerHTML = list.length
      ? list.map((r) => `<span class="gd-pill gd-true">${escapeHtml(r.rule)}${r.mode === "shadow" ? " (shadow)" : ""}</span>`).join(" ")
      : '<span class="gd-muted">nenhuma rule usa este fluxo ainda — escolha-o no campo <b>Fluxo</b> da rule em Config (editar)</span>';
  }

  function renderFieldList() {
    const dl = $("gd-flow-fields");
    const cals = Object.keys(S.data.calendars).map((k) => ["calendar." + k, "dentro do calendário " + k + " (true/false)"]);
    dl.innerHTML = FIELDS.concat(cals).map(([v, l]) => `<option value="${escapeHtml(v)}" label="${escapeHtml(l)}"></option>`).join("");
  }

  function renderCalendars() {
    const box = $("gd-flow-cal-list");
    const keys = Object.keys(S.data.calendars);
    if (!keys.length) {
      box.innerHTML = '<div class="gd-muted">Nenhum calendário. Crie um (ex: <b>comercial</b>, seg a sex 08:00–18:00) para usar “fora do horário comercial” nas condições.</div>';
      return;
    }
    if (box.contains(document.activeElement)) return; // não atrapalhar quem está digitando
    box.innerHTML = keys.map((k) => {
      const c = S.data.calendars[k];
      const days = DAYS.map((d) => `<label class="gd-flow-day"><input type="checkbox" data-gd-cal-day="${escapeHtml(k)}" value="${d}"${c.days.some((x) => WEEKDAY[lc(x)] === WEEKDAY[d]) ? " checked" : ""}> ${d}</label>`).join("");
      return `<div class="gd-row gd-flow-cal" data-cal="${escapeHtml(k)}">
        <div class="gd-field gd-field-tight"><label>Nome</label><input type="text" data-gd-cal-name="${escapeHtml(k)}" value="${escapeHtml(k)}"></div>
        <div class="gd-field"><label>Dias</label><div class="gd-flow-days">${days}</div></div>
        <div class="gd-field gd-field-tight" style="min-width:100px;"><label>Início</label><input type="time" data-gd-cal-field="start" data-cal="${escapeHtml(k)}" value="${escapeHtml(c.start)}"></div>
        <div class="gd-field gd-field-tight" style="min-width:100px;"><label>Fim</label><input type="time" data-gd-cal-field="end" data-cal="${escapeHtml(k)}" value="${escapeHtml(c.end)}"></div>
        <div class="gd-field"><label>Feriados <span class="gd-muted">(AAAA-MM-DD, separados por vírgula)</span></label><input type="text" data-gd-cal-field="holidays" data-cal="${escapeHtml(k)}" value="${escapeHtml(c.holidays.join(", "))}"></div>
        <div class="gd-field gd-field-tight" style="min-width:auto;"><label>&nbsp;</label><button type="button" class="gd-btn gd-btn-danger" data-gd-cal-remove="${escapeHtml(k)}">✕</button></div>
      </div>`;
    }).join("");
  }

  function testPath() {
    const nodes = new Set();
    const links = new Set();
    if (!S.testOpen) return { nodes, links };
    runTest(); // sempre com o estado atual (o desenho pode ser redesenhado antes do painel)
    if (!S.testResult) return { nodes, links };
    const tr = S.testResult.trace;
    tr.forEach((s, i) => {
      nodes.add(s.node);
      if (i + 1 < tr.length) links.add(s.node + "\u0000" + lc(s.port) + "\u0000" + tr[i + 1].node);
    });
    return { nodes, links };
  }

  function renderCanvas() {
    const fl = flow();
    while (svg.firstChild) svg.removeChild(svg.firstChild);
    const wrap = svg.parentNode;

    if (!fl) {
      svg.setAttribute("width", Math.max(600, wrap.clientWidth - 2));
      svg.setAttribute("height", 200);
      el("text", { x: 20, y: 40, class: "gd-flow-empty" }, svg).textContent = "Nenhum fluxo — clique em “+ Novo fluxo”.";
      return;
    }

    const maxX = Math.max(0, ...fl.nodes.map((n) => n.x + nodeWidth(n) / 2));
    const maxY = Math.max(0, ...fl.nodes.map((n) => n.y + nodeHeight(fl, n)));
    svg.setAttribute("width", Math.max(wrap.clientWidth - 2, maxX + 80));
    svg.setAttribute("height", Math.max(520, maxY + 90));

    const defs = el("defs", {}, svg);
    const mk = el("marker", { id: "gd-flow-arrow", viewBox: "0 0 10 10", refX: 9, refY: 5, markerWidth: 7, markerHeight: 7, orient: "auto-start-reverse" }, defs);
    el("path", { d: "M 0 0 L 10 5 L 0 10 z", class: "gd-flow-arrowhead" }, mk);
    const mk2 = el("marker", { id: "gd-flow-arrow-path", viewBox: "0 0 10 10", refX: 9, refY: 5, markerWidth: 7, markerHeight: 7, orient: "auto-start-reverse" }, defs);
    el("path", { d: "M 0 0 L 10 5 L 0 10 z", class: "gd-flow-arrowhead is-path" }, mk2);

    const path = testPath();

    const links = el("g", { class: "gd-flow-links" }, svg);
    fl.links.forEach((l, i) => {
      const a = nodeOf(fl, l.from), b = nodeOf(fl, l.to);
      if (!a || !b) return;
      const p = portPos(fl, a, linkPort(l));
      const d = curve(p.x, p.y, b.x, b.y);
      const onPath = path.links.has(l.from + "\u0000" + lc(linkPort(l)) + "\u0000" + l.to);
      let cls = "gd-flow-link";
      if (S.sel && S.sel.type === "link" && S.sel.idx === i) cls += " is-selected";
      if (onPath) cls += " is-path";
      el("path", { d, class: cls, "marker-end": `url(#${onPath ? "gd-flow-arrow-path" : "gd-flow-arrow"})` }, links);
      el("path", { d, class: "gd-flow-link-hit", "data-link": i }, links);
    });

    const nodes = el("g", { class: "gd-flow-nodes" }, svg);
    fl.nodes.forEach((n) => {
      const w = nodeWidth(n);
      const h = nodeHeight(fl, n);
      let cls = "gd-flow-node gd-flow-t-" + n.type;
      if (S.sel && S.sel.type === "node" && S.sel.id === n.id) cls += " is-selected";
      if (path.nodes.has(n.id)) cls += " is-path";
      if (n.type === "end" && n.action === "skip") cls += " is-skip";
      const g = el("g", { class: cls, transform: `translate(${n.x},${n.y})`, "data-node": n.id }, nodes);
      el("rect", { x: -w / 2, y: 0, width: w, height: h, rx: n.type === "start" || n.type === "end" ? 14 : 8, class: "gd-flow-box" }, g);
      el("rect", { x: -w / 2, y: 0, width: w, height: HEAD_H, rx: 8, class: "gd-flow-head" }, g);
      el("text", { x: -w / 2 + 10, y: 17, class: "gd-flow-title" }, g).textContent = trunc(`${TYPES[n.type] ? TYPES[n.type].icon : "?"} ${n.label || (TYPES[n.type] ? TYPES[n.type].name : n.type)}`, Math.floor(w / 7.4));
      summary(fl, n).forEach((line, i) => {
        el("text", { x: -w / 2 + 10, y: HEAD_H + 15 + i * LINE_H, class: "gd-flow-line" }, g).textContent = line;
      });
      if (n.type !== "start") el("circle", { cx: 0, cy: 0, r: 5, class: "gd-flow-port-in" }, g);

      const ps = ports(n);
      ps.forEach((p, i) => {
        const px = -w / 2 + ((i + 1) * w) / (ps.length + 1);
        const linked = !!linkFrom(fl, n.id, p.port);
        if (p.label) {
          el("text", { x: px, y: h - 12, "text-anchor": "middle", class: "gd-flow-port-label" }, g).textContent = trunc(p.label, 11);
        }
        el("circle", { cx: px, cy: h, r: 7, class: "gd-flow-port-out" + (linked ? " is-linked" : ""), "data-node": n.id, "data-port": p.port }, g);
      });
    });

    if (S.linking) {
      el("path", { d: curve(S.linking.x1, S.linking.y1, S.linking.x2, S.linking.y2), class: "gd-flow-link is-temp", id: "gd-flow-temp" }, svg);
    }
  }

  function curve(x1, y1, x2, y2) {
    const dy = Math.max(40, Math.abs(y2 - y1) / 2);
    return `M ${x1} ${y1} C ${x1} ${y1 + dy}, ${x2} ${y2 - dy}, ${x2} ${y2 - 6}`;
  }

  function opOptions(sel) {
    return OPS.map(([v, l]) => `<option value="${v}"${v === sel ? " selected" : ""}>${escapeHtml(l)}</option>`).join("");
  }
  function settableOptions(sel) {
    let html = SETTABLE.map(([v, l]) => `<option value="${v}"${v === sel ? " selected" : ""}>${escapeHtml(l)}</option>`).join("");
    if (sel && !SETTABLE_LABEL[sel]) html += `<option value="${escapeHtml(sel)}" selected>${escapeHtml(sel)} (inválido)</option>`;
    return html;
  }
  function varChips() {
    return `<div class="gd-flow-vars"><span class="gd-muted">Variáveis (clique para inserir):</span> ${VARS.map((v) => `<button type="button" class="gd-flow-var" data-gd-var="${escapeHtml(v)}">{${escapeHtml(v)}${v.endsWith(".") ? "…" : ""}}</button>`).join(" ")}</div>`;
  }

  function renderSide() {
    const box = $("gd-flow-side");
    const fl = flow();
    if (!fl) { box.innerHTML = ""; return; }

    if (S.sel && S.sel.type === "link" && fl.links[S.sel.idx]) {
      const l = fl.links[S.sel.idx];
      const a = nodeOf(fl, l.from), b = nodeOf(fl, l.to);
      const lab = (ports(a || {}).find((p) => lc(p.port) === lc(linkPort(l))) || {}).label;
      box.innerHTML = `<div class="gd-small-title">Ligação</div>
        <div><b>${escapeHtml(a ? a.label || a.id : l.from)}</b>${lab ? ` [${escapeHtml(lab)}]` : ""} → <b>${escapeHtml(b ? b.label || b.id : l.to)}</b></div>
        <div class="gd-actions"><button type="button" class="gd-btn gd-btn-danger" data-gd-flow="link-remove">Remover ligação</button></div>`;
      return;
    }

    const n = S.sel && S.sel.type === "node" ? nodeOf(fl, S.sel.id) : null;
    if (!n) {
      const errs = validate().filter((e) => e.indexOf(`Fluxo ${fl.name.trim() || "#"}`) === 0 || e.indexOf("Calendário") === 0);
      box.innerHTML = `<div class="gd-small-title">Fluxo</div>
        <div class="gd-field"><label>Nome <span class="gd-muted">(é o que a rule usa no campo Fluxo)</span></label><input type="text" data-gd-flow-prop="name" value="${escapeHtml(fl.name)}"></div>
        <div class="gd-field" style="margin-top:8px;"><label>Descrição</label><textarea rows="2" data-gd-flow-prop="description">${escapeHtml(fl.description)}</textarea></div>
        <div class="gd-muted" style="margin-top:8px;">${fl.nodes.length} blocos · ${fl.links.length} ligações</div>
        ${errs.length ? `<div class="gd-banner gd-err" style="margin-top:8px;">${errs.map(escapeHtml).join("<br>")}</div>` : ""}
        <div class="gd-muted" style="margin-top:10px;">
          <b>Arraste</b> os blocos. Para ligar, arraste da <b>bolinha de saída</b> (embaixo do bloco) até outro bloco.<br>
          Clique num bloco para editar; numa ligação para remover. <kbd>Delete</kbd> remove o selecionado.<br>
          Com um bloco selecionado, “Adicionar” já liga o novo bloco na primeira saída livre.<br>
          Saída sem ligação = <b>abre o chamado</b> com o que já foi definido.
        </div>`;
      return;
    }

    let html = `<div class="gd-small-title">${TYPES[n.type].icon} ${TYPES[n.type].name} <span class="gd-muted">(${escapeHtml(n.id)})</span></div>`;
    if (n.type !== "start") {
      html += `<div class="gd-field"><label>Título</label><input type="text" data-gd-node-prop="label" value="${escapeHtml(n.label)}" placeholder="${escapeHtml(TYPES[n.type].name)}"></div>`;
    }

    if (n.type === "start") {
      html += '<div class="gd-muted">Ponto de partida: o alerta chega aqui depois que a rule resolveu os campos. Ligue a saída no primeiro bloco.</div>';
    }

    if (n.type === "if") {
      html += `<div class="gd-field" style="margin-top:8px;"><label>Vale quando</label><select data-gd-node-prop="match">
          <option value="all"${n.match !== "any" ? " selected" : ""}>todas as regras (E)</option>
          <option value="any"${n.match === "any" ? " selected" : ""}>qualquer regra (OU)</option></select></div>`;
      html += '<div class="gd-small-title" style="margin-top:10px;">Regras</div>';
      n.conditions.forEach((c, i) => {
        html += `<div class="gd-flow-rule">
          <input type="text" list="gd-flow-fields" autocomplete="off" data-gd-cond="${i}" data-k="field" value="${escapeHtml(c.field)}" placeholder="campo">
          <select data-gd-cond="${i}" data-k="op">${opOptions(c.op)}</select>
          <input type="text" data-gd-cond="${i}" data-k="value" value="${escapeHtml(c.value)}" placeholder="valor"${NOVALUE_OPS.has(c.op) ? " disabled" : ""}>
          <button type="button" class="gd-btn gd-btn-danger gd-btn-xs" data-gd-cond-remove="${i}">✕</button></div>`;
      });
      html += '<div class="gd-actions"><button type="button" class="gd-btn" data-gd-flow="cond-add">+ Regra</button></div>';
      html += '<div class="gd-muted">Campo com vários valores (grupos, tags repetidas): “igual/contém/corresponde” vale se <b>algum</b> bater; “diferente/não…” se <b>nenhum</b> bater. Lista: valores separados por vírgula, aceita <code>*</code>.</div>';
    }

    if (n.type === "switch") {
      html += `<div class="gd-field" style="margin-top:8px;"><label>Campo</label><input type="text" list="gd-flow-fields" autocomplete="off" data-gd-node-prop="field" value="${escapeHtml(n.field)}" placeholder="host.tag.Criticidade"></div>`;
      html += '<div class="gd-small-title" style="margin-top:10px;">Casos <span class="gd-muted">(o primeiro que bater; nenhum = “outro”)</span></div>';
      n.cases.forEach((c, i) => {
        html += `<div class="gd-flow-rule"><input type="text" data-gd-case="${i}" value="${escapeHtml(c.value)}" placeholder="ex: Critico ou Alta,Crit*">
          <button type="button" class="gd-btn gd-btn-danger gd-btn-xs" data-gd-case-remove="${i}">✕</button></div>`;
      });
      html += '<div class="gd-actions"><button type="button" class="gd-btn" data-gd-flow="case-add">+ Caso</button></div>';
      html += '<div class="gd-muted">Sem diferenciar maiúsculas. Vários valores no mesmo caso: separe por vírgula. Aceita <code>*</code>.</div>';
    }

    if (n.type === "set") {
      html += '<div class="gd-small-title" style="margin-top:10px;">Campos</div>';
      n.set.forEach((a, i) => {
        const val = BOOL_FIELDS.has(a.field)
          ? `<select data-gd-set="${i}" data-k="value"><option value="true"${parseBool(a.value)[0] ? " selected" : ""}>sim</option><option value="false"${!parseBool(a.value)[0] ? " selected" : ""}>não</option></select>`
          : LONG_FIELDS.has(a.field)
            ? `<textarea rows="2" data-gd-set="${i}" data-k="value" placeholder="ex: LIGAR para o cliente ({host})">${escapeHtml(a.value)}</textarea>`
            : `<input type="text" data-gd-set="${i}" data-k="value" value="${escapeHtml(a.value)}">`;
        html += `<div class="gd-flow-rule gd-flow-setrow"><select data-gd-set="${i}" data-k="field">${settableOptions(a.field)}</select>${val}
          <button type="button" class="gd-btn gd-btn-danger gd-btn-xs" data-gd-set-remove="${i}">✕</button></div>`;
      });
      html += '<div class="gd-actions"><button type="button" class="gd-btn" data-gd-flow="set-add">+ Campo</button></div>';
      html += varChips();
      html += '<div class="gd-muted">Sobrescreve o que a rule definiria (inclusive tag do Zabbix). Texto que ficar vazio não apaga o valor da rule. A <b>instrução</b> vai no chamado (invisível pro solicitante) e no Teams.</div>';
    }

    if (n.type === "end") {
      html += `<div class="gd-field" style="margin-top:8px;"><label>Ação</label><select data-gd-node-prop="action">
          <option value="open"${n.action !== "skip" ? " selected" : ""}>Abrir chamado (fluxo normal da rule)</option>
          <option value="skip"${n.action === "skip" ? " selected" : ""}>Não abrir chamado (só comentar no evento)</option></select></div>`;
      if (n.action === "skip") {
        html += `<div class="gd-field" style="margin-top:8px;"><label>Comentário no evento <span class="gd-muted">(vazio = mostra o caminho)</span></label><textarea rows="2" data-gd-node-prop="comment">${escapeHtml(n.comment)}</textarea></div>`;
        html += varChips();
        html += '<div class="gd-muted">“Não abrir” só vale na abertura. Atualização e normalização de um chamado já aberto seguem normais.</div>';
      }
    }

    if (n.type !== "start") {
      html += '<div class="gd-actions"><button type="button" class="gd-btn gd-btn-danger" data-gd-flow="node-remove">Remover bloco</button></div>';
    }
    box.innerHTML = html;
  }

  // ---------- teste ----------

  function parseKV(text) {
    const out = [];
    String(text || "").split(/\r?\n/).forEach((line) => {
      const t = line.trim();
      if (!t) return;
      const i = t.indexOf("=");
      if (i < 0) out.push([t, ""]); else out.push([t.slice(0, i).trim(), t.slice(i + 1).trim()]);
    });
    return out;
  }

  function testInput(k) { return document.querySelector(`[data-gd-test="${k}"]`); }

  function testResolver() {
    const v = (k) => (testInput(k) ? testInput(k).value : "");
    const map = {};
    const add = (k, val) => { k = lc(k); (map[k] = map[k] || []).push(val); };
    ["host", "host.name", "trigger", "severity", "event.kind", "ticket.exists", "rule"].forEach((k) => { if (String(v(k)).trim() !== "") add(k, v(k)); });
    const sev = ["not classified", "information", "warning", "average", "high", "disaster"].indexOf(lc(v("severity")));
    if (sev >= 0) add("severity.level", String(sev));
    v("host.group").split(/\r?\n/).map((s) => s.trim()).filter(Boolean).forEach((g) => add("host.group", g));
    parseKV(v("host_tags")).forEach(([t, val]) => add("host.tag." + t, val));
    parseKV(v("event_tags")).forEach(([t, val]) => { add("event.tag." + t, val); add("tag." + t, val); });
    parseKV(v("extra")).forEach(([f, val]) => add(f, val));
    return (field) => {
      const k = lc(field);
      if (k === "host.groups") return map["host.group"] || [];
      return map[k] || [];
    };
  }

  function testNow() {
    const v = testInput("now") && testInput("now").value;
    const d = v ? new Date(v) : new Date();
    return isNaN(d.getTime()) ? new Date() : d;
  }

  function runTest() {
    const fl = flow();
    S.testResult = fl ? runFlow(S.data, fl, testResolver(), testNow()) : null;
  }

  function renderTest() {
    const box = $("gd-flow-test");
    box.style.display = S.testOpen ? "" : "none";
    if (!S.testOpen) return;
    runTest();
    const r = S.testResult;
    const out = $("gd-flow-test-result");
    if (!r) { out.innerHTML = ""; return; }
    const actionHtml = r.action === "skip"
      ? '<span class="gd-pill gd-false">NÃO abre chamado</span>'
      : '<span class="gd-pill gd-true">abre chamado</span>';
    const sets = r.set.length
      ? `<ul class="gd-flow-res">${r.set.map((a) => `<li><b>${escapeHtml(SETTABLE_LABEL[a.field] || a.field)}</b>: ${escapeHtml(a.value === "" ? "(vazio — mantém a rule)" : a.value)}</li>`).join("")}</ul>`
      : '<div class="gd-muted">Nenhum campo alterado — vale o que a rule define.</div>';
    const trace = `<ol class="gd-flow-trace">${r.trace.map((s) => {
      const fl = flow();
      const n = nodeOf(fl, s.node);
      const lab = n ? (ports(n).find((p) => lc(p.port) === lc(s.port)) || {}).label : "";
      return `<li><b>${escapeHtml(s.label || (TYPES[s.type] ? TYPES[s.type].name : s.type))}</b>${lab ? ` → <span class="gd-pill">${escapeHtml(lab)}</span>` : ""}${s.detail ? `<br><span class="gd-muted">${escapeHtml(s.detail)}</span>` : ""}</li>`;
    }).join("")}</ol>`;
    out.innerHTML = `<div class="gd-flow-test-grid">
        <div><div class="gd-small-title">Resultado</div>${actionHtml}
          ${r.error ? `<div class="gd-banner gd-err">${escapeHtml(r.error)} — no goDesk, a rule seguiria fixa.</div>` : ""}
          ${r.action === "skip" ? `<div class="gd-muted" style="margin-top:6px;">Comentário: ${escapeHtml(r.comment || "goDesk (fluxo " + r.flow + "): chamado não aberto — caminho: …")}</div>` : ""}
          <div class="gd-small-title" style="margin-top:10px;">Campos definidos pelo fluxo</div>${sets}</div>
        <div><div class="gd-small-title">Caminho percorrido</div>${trace}</div>
      </div>`;
  }

  // =========================================================
  // interação no canvas
  // =========================================================

  function svgPoint(evt) {
    const pt = svg.createSVGPoint();
    pt.x = evt.clientX;
    pt.y = evt.clientY;
    return pt.matrixTransform(svg.getScreenCTM().inverse());
  }

  function onPointerDown(e) {
    const fl = flow();
    if (!fl) return;
    const port = e.target.closest(".gd-flow-port-out");
    if (port) {
      const n = nodeOf(fl, port.getAttribute("data-node"));
      const p = portPos(fl, n, port.getAttribute("data-port"));
      S.linking = { from: n.id, port: port.getAttribute("data-port"), x1: p.x, y1: p.y, x2: p.x, y2: p.y + 30 };
      svg.setPointerCapture(e.pointerId);
      e.preventDefault();
      renderCanvas();
      return;
    }
    const g = e.target.closest(".gd-flow-node");
    if (g) {
      const n = nodeOf(fl, g.getAttribute("data-node"));
      const p = svgPoint(e);
      S.drag = { id: n.id, dx: p.x - n.x, dy: p.y - n.y, moved: false };
      svg.setPointerCapture(e.pointerId);
      e.preventDefault();
      return;
    }
    const hit = e.target.closest(".gd-flow-link-hit");
    if (hit) {
      S.sel = { type: "link", idx: +hit.getAttribute("data-link") };
      renderCanvas(); renderSide();
      return;
    }
    S.sel = null;
    renderCanvas(); renderSide();
  }

  function onPointerMove(e) {
    if (S.linking) {
      const p = svgPoint(e);
      S.linking.x2 = p.x; S.linking.y2 = p.y;
      const t = $("gd-flow-temp");
      if (t) t.setAttribute("d", curve(S.linking.x1, S.linking.y1, p.x, p.y));
      return;
    }
    if (!S.drag) return;
    const fl = flow();
    const n = nodeOf(fl, S.drag.id);
    if (!n) return;
    const p = svgPoint(e);
    const x = Math.max(nodeWidth(n) / 2 + 4, Math.round((p.x - S.drag.dx) / 5) * 5);
    const y = Math.max(6, Math.round((p.y - S.drag.dy) / 5) * 5);
    if (x !== n.x || y !== n.y) {
      n.x = x; n.y = y;
      S.drag.moved = true;
      dirty();
      renderCanvas();
    }
  }

  function onPointerUp(e) {
    try { svg.releasePointerCapture(e.pointerId); } catch (err) { /* já liberado */ }
    const fl = flow();
    if (S.linking) {
      const L = S.linking;
      S.linking = null;
      const under = document.elementFromPoint(e.clientX, e.clientY);
      const g = under && under.closest ? under.closest(".gd-flow-node") : null;
      if (g && fl) {
        const to = g.getAttribute("data-node");
        const target = nodeOf(fl, to);
        if (to !== L.from && target && target.type !== "start") {
          setLink(fl, L.from, L.port, to);
        }
      }
      renderCanvas(); renderSide(); renderTest();
      return;
    }
    if (!S.drag) return;
    const d = S.drag;
    S.drag = null;
    if (!d.moved) S.sel = { type: "node", id: d.id };
    renderCanvas(); renderSide();
  }

  function setLink(fl, from, port, to) {
    fl.links = fl.links.filter((l) => !(l.from === from && lc(linkPort(l)) === lc(port)));
    const l = { from, to };
    if (port && port !== "next") l.port = port;
    fl.links.push(l);
    dirty();
  }

  function removeNode(fl, id) {
    fl.nodes = fl.nodes.filter((n) => n.id !== id);
    fl.links = fl.links.filter((l) => l.from !== id && l.to !== id);
    dirty();
  }

  function deleteSelection() {
    const fl = flow();
    if (!fl || !S.sel) return;
    if (S.sel.type === "node") {
      const n = nodeOf(fl, S.sel.id);
      if (!n || n.type === "start") return;
      if (!confirm(`Remover o bloco ${n.label || n.id} e as ligações dele?`)) return;
      removeNode(fl, n.id);
    }
    else if (S.sel.type === "link") {
      fl.links.splice(S.sel.idx, 1);
      dirty();
    }
    S.sel = null;
    renderCanvas(); renderSide(); renderTest();
  }

  function addBlock(type) {
    const fl = flow();
    if (!fl) { alert("Crie um fluxo antes."); return; }
    let pos = freeSpot(fl);
    let from = null;
    const sel = S.sel && S.sel.type === "node" ? nodeOf(fl, S.sel.id) : null;
    if (sel) {
      const free = ports(sel).find((p) => !linkFrom(fl, sel.id, p.port));
      if (free) {
        from = { id: sel.id, port: free.port };
        const pp = portPos(fl, sel, free.port);
        pos = { x: Math.max(NODE_W / 2 + 10, pp.x), y: pp.y + 60 };
      }
    }
    const n = newNode(fl, type, pos.x, pos.y);
    fl.nodes.push(n);
    if (from) setLink(fl, from.id, from.port, n.id);
    S.sel = { type: "node", id: n.id };
    dirty();
    renderCanvas(); renderSide(); renderTest();
  }

  // renomear caso do switch: a ligação acompanha
  function renameCase(fl, n, i, value) {
    const old = n.cases[i].value;
    n.cases[i].value = value;
    fl.links.forEach((l) => { if (l.from === n.id && l.port && lc(l.port) === lc(old)) l.port = value; });
  }

  function renameCalendar(oldName, newName) {
    newName = newName.trim();
    if (!newName || newName === oldName) return false;
    if (Object.keys(S.data.calendars).some((k) => lc(k) === lc(newName) && k !== oldName)) { alert("Já existe um calendário com esse nome."); return false; }
    const c = S.data.calendars[oldName];
    delete S.data.calendars[oldName];
    S.data.calendars[newName] = c;
    const re = new RegExp("calendar\\." + oldName.replace(/[.*+?^${}()|[\]\\]/g, "\\$&") + "(?![\\w-])", "gi");
    const fix = (s) => String(s).replace(re, "calendar." + newName);
    S.data.flows.forEach((f) => f.nodes.forEach((n) => {
      if (n.type === "if") n.conditions.forEach((c2) => { c2.field = fix(c2.field); c2.value = fix(c2.value); });
      if (n.type === "switch") n.field = fix(n.field);
      if (n.type === "set") n.set.forEach((a) => { a.value = fix(a.value); });
      if (n.type === "end") n.comment = fix(n.comment);
    }));
    return true;
  }

  // ---------- dados do Zabbix (simulador) ----------

  function readJson(r) {
    return r.text().then((text) => {
      try { return JSON.parse(text); }
      catch (e) {
        const plain = text.replace(/<[^>]*>/g, " ").replace(/\s+/g, " ").trim().slice(0, 200);
        throw new Error(`resposta não é JSON (HTTP ${r.status}). Recarregue a página. Início da resposta: ${plain}`);
      }
    });
  }

  function fetchData(params) {
    return fetch("zabbix.php?action=godesk.flows.data", {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: new URLSearchParams(params),
    }).then(readJson);
  }

  function loadRecent() {
    if (S.recentLoaded) return;
    S.recentLoaded = true;
    const sel = $("gd-flow-test-recent");
    sel.innerHTML = '<option value="">carregando…</option>';
    fetchData({ op: "recent" })
      .then((d) => {
        if (d.error) throw new Error(d.error);
        sel.innerHTML = '<option value="">— escolha um problema —</option>' + (d.events || []).map((e) =>
          `<option value="${escapeHtml(e.eventid)}">${escapeHtml(e.clock)} · ${escapeHtml(e.host)} · ${escapeHtml(trunc(e.name, 70))}</option>`).join("");
      })
      .catch((err) => { S.recentLoaded = false; sel.innerHTML = '<option value="">falha ao carregar</option>'; alert("Falha ao listar problemas: " + err.message); });
  }

  function loadEvent(eventid) {
    eventid = String(eventid || "").trim();
    if (!eventid) return;
    fetchData({ op: "event", eventid })
      .then((d) => {
        if (d.error) { alert(d.error); return; }
        const ev = d.event || {};
        const set = (k, v) => { const i = testInput(k); if (i) i.value = v == null ? "" : v; };
        set("host", ev.host); set("host.name", ev["host.name"]); set("trigger", ev.trigger);
        const sev = testInput("severity");
        if (sev && ![...sev.options].some((o) => o.value === ev.severity)) sev.add(new Option(ev.severity, ev.severity));
        set("severity", ev.severity);
        set("event.kind", ev["event.kind"]);
        set("now", ev.clock);
        set("host.group", (ev["host.group"] || []).join("\n"));
        set("host_tags", (ev.host_tags || []).map((t) => t.tag + "=" + t.value).join("\n"));
        set("event_tags", (ev.event_tags || []).map((t) => t.tag + "=" + t.value).join("\n"));
        $("gd-flow-test-eventid").value = eventid;
        renderCanvas(); renderTest();
      })
      .catch((err) => alert("Falha ao carregar o evento: " + err.message));
  }

  // ---------- export / import ----------

  function exportFile() {
    const text = $("gd-flow-export-text").value;
    if (!text.trim()) { alert("Nenhum fluxo salvo para exportar."); return; }
    const blob = new Blob([text], { type: "text/yaml" });
    const a = document.createElement("a");
    a.href = URL.createObjectURL(blob);
    a.download = "godesk-flows.yaml";
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(a.href);
  }

  function importFile(input) {
    const file = input.files && input.files[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = () => {
      input.value = "";
      if (!confirm(`Importar "${file.name}"? Isso SUBSTITUI todos os fluxos e calendários (um backup do arquivo atual é mantido no servidor).`)) return;
      const form = $("gd-flow-import-form");
      form.querySelector('textarea[name="import_yaml"]').value = String(reader.result || "");
      S.dirty = false;
      form.submit();
    };
    reader.readAsText(file);
  }

  // =========================================================
  // init
  // =========================================================

  function init() {
    const box = $("gd-flow");
    const dataEl = $("gd-flow-data");
    if (!box || !dataEl) return;

    let payload = {};
    try { payload = JSON.parse(dataEl.textContent || "{}"); } catch (e) { payload = {}; }

    S = {
      data: prepData(payload.data || {}),
      rulesByFlow: payload.rules_by_flow || {},
      cur: 0,
      sel: null,
      drag: null,
      linking: null,
      dirty: false,
      testOpen: false,
      testResult: null,
      recentLoaded: false,
      lastVarTarget: null,
    };

    const last = storageGet("gd-flow-cur");
    const idx = S.data.flows.findIndex((f) => f.name === last);
    if (idx >= 0) S.cur = idx;

    svg = $("gd-flow-svg");
    svg.addEventListener("pointerdown", onPointerDown);
    svg.addEventListener("pointermove", onPointerMove);
    svg.addEventListener("pointerup", onPointerUp);
    svg.addEventListener("pointercancel", onPointerUp);

    const root = document.querySelector(".godesk-module");

    root.addEventListener("click", (e) => {
      const add = e.target.closest("[data-gd-flow-add]");
      if (add) { addBlock(add.dataset.gdFlowAdd); return; }

      const v = e.target.closest("[data-gd-var]");
      if (v) {
        const t = S.lastVarTarget;
        if (t && document.body.contains(t)) {
          const ins = "{" + v.dataset.gdVar + "}";
          const pos = t.selectionStart == null ? t.value.length : t.selectionStart;
          t.value = t.value.slice(0, pos) + ins + t.value.slice(t.selectionEnd == null ? pos : t.selectionEnd);
          t.focus();
          const caret = v.dataset.gdVar.endsWith(".") ? pos + ins.length - 1 : pos + ins.length;
          t.setSelectionRange(caret, caret);
          t.dispatchEvent(new Event("input", { bubbles: true }));
        }
        else alert("Clique antes no campo de texto onde quer inserir a variável.");
        return;
      }

      const calRm = e.target.closest("[data-gd-cal-remove]");
      if (calRm) {
        const k = calRm.dataset.gdCalRemove;
        if (confirm(`Remover o calendário ${k}? Condições que usam calendar.${k} vão precisar ser ajustadas.`)) {
          delete S.data.calendars[k]; dirty(); renderAll();
        }
        return;
      }

      const fl = flow();
      const n = fl && S.sel && S.sel.type === "node" ? nodeOf(fl, S.sel.id) : null;
      const rm = e.target.closest("[data-gd-cond-remove],[data-gd-case-remove],[data-gd-set-remove]");
      if (rm && n) {
        if (rm.dataset.gdCondRemove !== undefined) n.conditions.splice(+rm.dataset.gdCondRemove, 1);
        if (rm.dataset.gdCaseRemove !== undefined) {
          const c = n.cases[+rm.dataset.gdCaseRemove];
          fl.links = fl.links.filter((l) => !(l.from === n.id && l.port && lc(l.port) === lc(c.value)));
          n.cases.splice(+rm.dataset.gdCaseRemove, 1);
        }
        if (rm.dataset.gdSetRemove !== undefined) n.set.splice(+rm.dataset.gdSetRemove, 1);
        dirty(); renderCanvas(); renderSide(); renderTest();
        return;
      }

      const act = e.target.closest("[data-gd-flow]");
      if (!act) return;
      const a = act.dataset.gdFlow;
      if (a === "flow-add") {
        const name = (prompt("Nome do fluxo (ex: infra):") || "").trim();
        if (!name) return;
        if (S.data.flows.some((f) => lc(f.name) === lc(name))) { alert("Já existe um fluxo com esse nome."); return; }
        S.data.flows.push(newFlow(name));
        S.cur = S.data.flows.length - 1; S.sel = null; dirty(); renderAll();
      }
      else if (a === "flow-dup" && fl) {
        let name = fl.name + " (cópia)";
        while (S.data.flows.some((f) => lc(f.name) === lc(name))) name += "+";
        const copy = JSON.parse(JSON.stringify(fl));
        copy.name = name;
        S.data.flows.push(copy);
        S.cur = S.data.flows.length - 1; S.sel = null; dirty(); renderAll();
      }
      else if (a === "flow-remove" && fl) {
        const users = S.rulesByFlow[lc(fl.name)] || [];
        if (!confirm(`Remover o fluxo ${fl.name}?` + (users.length ? `\nAs rules ${users.map((u) => u.rule).join(", ")} usam ele e passarão a seguir fixas.` : ""))) return;
        S.data.flows.splice(S.cur, 1);
        S.cur = Math.max(0, S.cur - 1); S.sel = null; dirty(); renderAll();
      }
      else if (a === "cal-add") {
        let name = Object.keys(S.data.calendars).length ? "calendario" : "comercial";
        let i = 2;
        while (S.data.calendars[name]) name = "calendario" + i++;
        S.data.calendars[name] = { days: ["seg", "ter", "qua", "qui", "sex"], start: "08:00", end: "18:00", holidays: [] };
        dirty(); renderAll();
      }
      else if (a === "layout" && fl) { autoLayout(fl); dirty(); renderCanvas(); }
      else if (a === "test-toggle") { S.testOpen = !S.testOpen; if (S.testOpen && !testInput("now").value) testInput("now").value = fmtDate(new Date()) + "T" + fmtTime(new Date()); renderCanvas(); renderTest(); }
      else if (a === "test-load") loadEvent($("gd-flow-test-eventid").value);
      else if (a === "node-remove") deleteSelection();
      else if (a === "link-remove") deleteSelection();
      else if (a === "cond-add" && n) { n.conditions.push({ field: "", op: "eq", value: "" }); dirty(); renderCanvas(); renderSide(); }
      else if (a === "case-add" && n) {
        let v = "valor"; let i = 2;
        while (n.cases.some((c) => lc(c.value) === lc(v))) v = "valor" + i++;
        n.cases.push({ value: v }); dirty(); renderCanvas(); renderSide();
      }
      else if (a === "set-add" && n) {
        const used = new Set(n.set.map((x) => x.field));
        const f = (SETTABLE.find(([k]) => !used.has(k)) || ["instrucao"])[0];
        n.set.push({ field: f, value: BOOL_FIELDS.has(f) ? "true" : "" }); dirty(); renderCanvas(); renderSide();
      }
    });

    root.addEventListener("focusin", (e) => {
      const t = e.target;
      if ((t.tagName === "INPUT" && t.type === "text") || t.tagName === "TEXTAREA") {
        if (t.closest("#gd-flow-side")) S.lastVarTarget = t;
      }
    });

    root.addEventListener("input", (e) => {
      const t = e.target;
      const fl = flow();
      const n = fl && S.sel && S.sel.type === "node" ? nodeOf(fl, S.sel.id) : null;

      if (t.dataset.gdFlowProp && fl) {
        fl[t.dataset.gdFlowProp] = t.value;
        dirty();
        if (t.dataset.gdFlowProp === "name") { renderFlowSelect(); renderRules(); }
        return;
      }
      if (t.dataset.gdNodeProp && n && t.tagName !== "SELECT") {
        n[t.dataset.gdNodeProp] = t.value; dirty(); renderCanvas(); renderTest();
        return;
      }
      if (t.dataset.gdCond !== undefined && n && t.tagName !== "SELECT") {
        n.conditions[+t.dataset.gdCond][t.dataset.k] = t.value; dirty(); renderCanvas(); renderTest();
        return;
      }
      if (t.dataset.gdCase !== undefined && n) {
        renameCase(fl, n, +t.dataset.gdCase, t.value); dirty(); renderCanvas(); renderTest();
        return;
      }
      if (t.dataset.gdSet !== undefined && n && t.tagName !== "SELECT") {
        n.set[+t.dataset.gdSet][t.dataset.k] = t.value; dirty(); renderCanvas(); renderTest();
        return;
      }
      if (t.dataset.gdCalField) {
        const c = S.data.calendars[t.dataset.cal];
        if (!c) return;
        if (t.dataset.gdCalField === "holidays") c.holidays = t.value.split(",").map((s) => s.trim()).filter(Boolean);
        else c[t.dataset.gdCalField] = t.value;
        dirty(); renderTest();
        return;
      }
      if (t.dataset.gdTest !== undefined) { renderCanvas(); renderTest(); }
    });

    root.addEventListener("change", (e) => {
      const t = e.target;
      const fl = flow();
      const n = fl && S.sel && S.sel.type === "node" ? nodeOf(fl, S.sel.id) : null;

      if (t.id === "gd-flow-select") {
        S.cur = +t.value || 0; S.sel = null;
        storageSet("gd-flow-cur", (flow() || {}).name || "");
        renderAll();
        return;
      }
      if (t.id === "gd-flow-test-recent") { if (t.value) loadEvent(t.value); return; }
      if (t.id === "gd-flow-import-file") { importFile(t); return; }
      if (t.dataset.gdNodeProp && n && t.tagName === "SELECT") {
        n[t.dataset.gdNodeProp] = t.value; dirty(); renderCanvas(); renderSide(); renderTest();
        return;
      }
      if (t.dataset.gdCond !== undefined && n && t.tagName === "SELECT") {
        n.conditions[+t.dataset.gdCond][t.dataset.k] = t.value; dirty(); renderCanvas(); renderSide(); renderTest();
        return;
      }
      if (t.dataset.gdSet !== undefined && n && t.tagName === "SELECT") {
        const a = n.set[+t.dataset.gdSet];
        a[t.dataset.k] = t.value;
        if (t.dataset.k === "field" && BOOL_FIELDS.has(t.value) && !parseBool(a.value)[1]) a.value = "true";
        dirty(); renderCanvas(); renderSide(); renderTest();
        return;
      }
      if (t.dataset.gdCalName !== undefined) {
        if (!renameCalendar(t.dataset.gdCalName, t.value)) t.value = t.dataset.gdCalName;
        else { dirty(); t.blur(); renderAll(); }
        return;
      }
      if (t.dataset.gdCalDay !== undefined) {
        const c = S.data.calendars[t.dataset.gdCalDay];
        if (!c) return;
        const boxes = document.querySelectorAll(`[data-gd-cal-day="${CSS.escape(t.dataset.gdCalDay)}"]`);
        c.days = [...boxes].filter((b) => b.checked).map((b) => b.value);
        dirty(); renderTest();
        return;
      }
      if (t.dataset.gdTest !== undefined) { renderCanvas(); renderTest(); }
    });

    $("gd-flow-test-recent").addEventListener("focus", loadRecent);
    $("gd-flow-test-recent").addEventListener("mousedown", loadRecent);
    $("gd-flow-test-eventid").addEventListener("keydown", (e) => { if (e.key === "Enter") { e.preventDefault(); loadEvent(e.target.value); } });

    document.addEventListener("keydown", (e) => {
      if (e.key !== "Delete" && e.key !== "Backspace") return;
      const t = document.activeElement;
      if (t && /^(INPUT|TEXTAREA|SELECT)$/.test(t.tagName)) return;
      if (S.sel) { e.preventDefault(); deleteSelection(); }
    });

    document.querySelector("[data-gd-flow-export]").addEventListener("click", exportFile);
    document.querySelector("[data-gd-flow-import]").addEventListener("click", () => $("gd-flow-import-file").click());

    $("gd-flow-form").addEventListener("submit", (e) => {
      const errs = validate();
      if (errs.length) {
        e.preventDefault();
        alert("Corrija antes de salvar:\n\n" + errs.join("\n"));
        return;
      }
      $("gd-flow-json").value = JSON.stringify(cleanData());
      storageSet("gd-flow-cur", (flow() || {}).name || "");
      S.dirty = false;
    });

    window.addEventListener("beforeunload", (e) => {
      if (S.dirty) { e.preventDefault(); e.returnValue = ""; }
    });
    window.addEventListener("resize", () => renderCanvas());

    renderAll();
  }

  // exposto só pra testes (Node) do port do motor
  if (typeof window !== "undefined") window.__gdFlowEngine = { runFlow, evalCondition, wildcard, inCalendar };

  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", init);
  else init();
})();
