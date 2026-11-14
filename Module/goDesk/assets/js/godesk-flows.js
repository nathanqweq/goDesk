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
    branch: { icon: "⑃", name: "Condição múltipla" },
    transform: { icon: "ƒ", name: "Transformar texto" },
    table: { icon: "▦", name: "Tabela" },
    update: { icon: "⤴", name: "Atualizar chamado" },
    note: { icon: "✉", name: "Texto no chamado" },
    zabbix: { icon: "⚑", name: "Ação no Zabbix" },
    http: { icon: "⇄", name: "HTTP" },
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
    ["starts", "começa com"], ["ends", "termina com"],
    ["regex", "regex"], ["not_regex", "não casa a regex"],
    ["between", "entre (início..fim)"],
  ];
  const TRANSFORM_OPS = [
    ["set", "texto (com variáveis)"], ["extract", "extrair por regex"], ["replace", "substituir"],
    ["upper", "MAIÚSCULAS"], ["lower", "minúsculas"], ["trim", "aparar espaços"], ["default", "valor padrão se vazio"],
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
    ["send_teams", "Enviar Teams"], ["teams_webhook", "Webhook(s) do Teams (vírgula separa)"],
    ["teams_title", "Título do card do Teams"], ["teams_message", "Texto extra no card do Teams"],
    ["repeat_update", "Registrar \"Recebemos novamente\" no reenvio"],
    ["send_email", "Enviar e-mail"], ["email_to", "E-mail para"], ["email_cc", "E-mail cópia"], ["once_per_day", "E-mail 1x por dia"],
    ["send_more_info", "Enviar mais informações"], ["more_info_text", "Texto de mais informações"],
    ["autoclose", "Autoclose"], ["dependency", "Dependência"], ["dependency_version", "Versão da dependência (v1/v2/shadow)"],
    ["custom_status", "Custom status"], ["status_open", "Status abertura (ID)"], ["status_update", "Status atualização (ID)"],
  ];
  const SETTABLE_LABEL = Object.fromEntries(SETTABLE);
  const BOOL_FIELDS = new Set(["autoclose", "dependency", "custom_status", "send_teams", "send_email", "once_per_day", "send_more_info", "repeat_update"]);
  const LONG_FIELDS = new Set(["instrucao", "more_info_text", "teams_message"]);
  // mesma lista do config.FlowUpdateFields
  const UPDATE_FIELDS = [["urgency", "Urgência"], ["impact", "Impacto"], ["priority", "Prioridade"], ["oper_group", "Grupo operador (ID)"], ["operator", "Operador (ID)"]];
  const HTTP_METHODS = ["POST", "GET", "PUT", "PATCH", "DELETE"];
  const PORT_LABEL = { true: "sim", false: "não", default: "outro", found: "achou", not_found: "não achou", else: "senão", next: "" };
  const MAX_COND_DEPTH = 3;
  const SEVERITIES = [["", "não muda"], ["0", "Não classificada"], ["1", "Informação"], ["2", "Atenção"], ["3", "Média"], ["4", "Alta"], ["5", "Desastre"]];
  const NOTE_TARGETS = [["internal", "Ação interna (invisível ao solicitante)"], ["public", "Ação visível ao solicitante"], ["description", "Descrição do chamado (na abertura)"]];

  // igual ao config.SeverityLevel do Go
  function severityLevel(s) {
    switch (lc(s)) {
      case "0": case "not classified": case "não classificada": case "nao classificada": return 0;
      case "1": case "information": case "informação": case "informacao": return 1;
      case "2": case "warning": case "atenção": case "atencao": case "aviso": return 2;
      case "3": case "average": case "média": case "media": return 3;
      case "4": case "high": case "alta": return 4;
      case "5": case "disaster": case "desastre": return 5;
    }
    return -1;
  }

  // mesmos campos do eventResolver (internal/app/flow.go) + os do motor
  const FIELDS = [
    ["host", "host (nome técnico)"], ["host.name", "host (nome visível)"], ["host.group", "grupos do host"],
    ["host.tag.", "tag do host — host.tag.<nome>"], ["event.tag.", "tag do evento — event.tag.<nome>"],
    ["trigger", "nome do trigger/problema"], ["severity", "severidade (nome)"], ["severity.level", "severidade 0 a 5"],
    ["event.kind", "problem / recovery"], ["status", "status (PROBLEM/RESOLVED)"], ["value", "valor do item"],
    ["rule", "rule"], ["cliente", "cliente"], ["equipe", "equipe"],
    ["ticket.exists", "chamado já existe (true/false)"], ["ticket.status", "status do chamado"], ["ticket.number", "número do chamado (HTTP)"],
    ["event.age", "minutos desde o início do problema"], ["event.resends", "quantas vezes o alerta já chegou antes (0 = primeira)"],
    ["event.acked_by_user", "reconhecido pelo atendimento (true/false)"], ["event.acknowledged", "reconhecido por qualquer um, inclusive o goDesk"],
    ["now.weekday", "dia da semana (dom..sab)"], ["now.hour", "hora (00-23)"], ["now.time", "hora HH:MM"], ["now.date", "data AAAA-MM-DD"],
    ["set.", "campo já definido no fluxo — set.<campo>"],
    ["var.", "variável criada no Transformar texto — var.<nome>"],
    ["host.inventory.", "inventário do host — host.inventory.<campo> (location, contact, os...)"],
    ["host.macro.", "macro do host — host.macro.<NOME> ({$NOME})"],
    ["item:", "último valor de um item do host — item:<chave> (ex: item:icmpping)"],
    ["host.problems", "problemas ativos do host agora (número)"],
    ["host.problems.", "problemas do host com severidade ≥ nível — host.problems.4"],
    ["group.problems.", "problemas ativos de um grupo — group.problems.<grupo> (aceita *)"],
  ];
  const VARS = ["host", "host.name", "trigger", "severity", "value", "host.tag.", "event.tag.", "cliente", "now.time", "event.age", "ticket.number"];

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
      case "starts": return any((v) => String(v).toLowerCase().startsWith(String(want).toLowerCase()));
      case "ends": return any((v) => String(v).toLowerCase().endsWith(String(want).toLowerCase()));
      case "regex": { const re = flowRegex(want); return re ? any((v) => re.test(String(v))) : false; }
      case "not_regex": return flowRegex(want) ? !evalCondition(vals, "regex", want) : false;
      case "between": {
        const p = betweenParts(want);
        if (!p) return false;
        return any((v) => between(String(v).trim(), p[0], p[1]));
      }
    }
    return false;
  }

  // regex sem diferenciar maiúsculas (igual ao config.CompileFlowRegex);
  // inválida = null
  function flowRegex(p) {
    // "u": escape inválido vira erro (como no RE2 do Go) e . / maiúsculas
    // seguem Unicode, igual ao Go
    try { return new RegExp(String(p), "iu"); } catch (e) { return null; }
  }

  function betweenParts(v) {
    v = String(v);
    const i = v.indexOf("..");
    if (i < 0) return null;
    const lo = v.slice(0, i).trim(), hi = v.slice(i + 2).trim();
    return lo !== "" && hi !== "" ? [lo, hi] : null;
  }

  // igual ao between do Go: número compara como número, o resto como texto;
  // início > fim atravessa a virada (22:00..06:00)
  function between(v, lo, hi) {
    if (v === "") return false;
    const fv = num(v), flo = num(lo), fhi = num(hi);
    if (!isNaN(fv) && !isNaN(flo) && !isNaN(fhi)) {
      return flo <= fhi ? fv >= flo && fv <= fhi : fv >= flo || fv <= fhi;
    }
    return lo <= hi ? v >= lo && v <= hi : v >= lo || v <= hi;
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

  // "campo|filtro" → [campo, filtro] (igual ao config.SplitTemplateVar)
  function splitVar(s) {
    s = String(s).trim();
    const i = s.lastIndexOf("|");
    return i >= 0 ? [s.slice(0, i).trim(), s.slice(i + 1).trim().toLowerCase()] : [s, ""];
  }

  // igual ao url.QueryEscape do Go (espaço vira +; !'()* codificados)
  function queryEscape(s) {
    let out;
    try { out = encodeURIComponent(s); } catch (e) { out = encodeURIComponent(String(s).replace(/[\uD800-\uDFFF]/g, "�")); }
    return out.replace(/%20/g, "+").replace(/[!'()*]/g, (c) => "%" + c.charCodeAt(0).toString(16).toUpperCase());
  }

  // igual ao flow.ApplyFilter do Go
  function applyFilter(v, filter) {
    if (filter === "json") { const bs = String.fromCharCode(92); return JSON.stringify(String(v)).slice(1, -1).split(String.fromCharCode(0x2028)).join(bs + "u2028").split(String.fromCharCode(0x2029)).join(bs + "u2029"); }
    if (filter === "url") return queryEscape(String(v));
    return v;
  }

  function runFlow(data, fl, resolve, now) {
    const r = { flow: fl.name, action: "open", comment: "", set: [], updates: [], http: [], zabbix: [], notes: [], vars: {}, trace: [], error: "" };
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
      if (lower.startsWith("var.")) { const k = lower.slice(4); return Object.prototype.hasOwnProperty.call(r.vars, k) ? [r.vars[k]] : []; }
      if (lower.startsWith("set.")) { const v = valueOf(lower.slice(4)); return v == null ? [] : [v]; }
      return resolve(field) || [];
    };
    const render = (tpl) => {
      tpl = String(tpl == null ? "" : tpl);
      if (tpl.indexOf("{") < 0) return tpl;
      return tpl.replace(/\{([^{}]+)\}/g, (m, inner) => {
        const [f, filter] = splitVar(inner);
        return applyFilter(nonEmpty(values(f)).join(", "), filter);
      });
    };

    // regras com E/OU; grupo avalia recursivamente (igual ao evalConds do Go)
    function evalConds(match, conds) {
      const anyMode = match === "any";
      let result = !anyMode;
      const parts = [];
      conds.forEach((c) => {
        let ok;
        if (c.conditions && c.conditions.length) {
          const [gok, gdetail] = evalConds(c.match, c.conditions);
          ok = gok;
          parts.push(`${ok ? "✓" : "✗"} (${gdetail})`);
        }
        else {
          const vals = values(c.field);
          const want = render(c.value);
          ok = evalCondition(vals, c.op || "eq", want);
          parts.push(`${ok ? "✓" : "✗"} ${c.field} ${c.op || "eq"}${want !== "" ? " " + want : ""} (${showValues(vals)})`);
        }
        if (anyMode && ok) result = true;
        if (!anyMode && !ok) result = false;
      });
      return [result, parts.join(anyMode ? " OU " : " E ")];
    }

    // igual ao transform do Go
    function transform(t) {
      const src = render(t.from);
      switch (t.op) {
        case "extract": {
          const re = flowRegex(t.arg);
          if (!re) return "";
          const m = src.match(re);
          if (!m) return "";
          return m.length > 1 ? (m[1] == null ? "" : m[1]) : m[0];
        }
        case "replace": return t.arg === "" ? src : src.split(t.arg).join(render(t.arg2 || ""));
        case "upper": return src.toUpperCase();
        case "lower": return src.toLowerCase();
        case "trim": return src.trim();
        case "default": return src.trim() === "" ? render(t.arg || "") : src;
      }
      return src;
    }

    let cur = fl.nodes.find((n) => n.type === "start");
    if (!cur) { r.error = "fluxo sem bloco de início"; return r; }

    for (let i = 0; i < MAX_STEPS; i++) {
      const step = { node: cur.id, type: cur.type, label: cur.label || "", port: "", detail: "" };
      let port = "";

      if (cur.type === "start") {
        port = "next";
      }
      else if (cur.type === "if") {
        const [ok, detail] = evalConds(cur.match, cur.conditions || []);
        step.detail = detail;
        port = ok ? "true" : "false";
      }
      else if (cur.type === "branch") {
        port = "else";
        const parts = [];
        for (const b of cur.branches || []) {
          const [ok, detail] = evalConds(b.match, b.conditions || []);
          parts.push(`${ok ? "✓" : "✗"} ${b.label || b.id}: ${detail}`);
          if (ok) { port = b.id; break; }
        }
        step.detail = parts.join(" | ");
      }
      else if (cur.type === "transform") {
        const done = [];
        (cur.transforms || []).forEach((t) => {
          const v = transform(t);
          r.vars[String(t.var).toLowerCase()] = v;
          done.push("var." + t.var + "=" + v);
        });
        step.detail = done.join("; ");
        port = "next";
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
      else if (cur.type === "table") {
        const vals = values(cur.field);
        port = "not_found";
        step.detail = `${cur.field} = ${showValues(vals)} → nenhuma linha`;
        for (const row of cur.rows || []) {
          if (!evalCondition(vals, "in", row.match)) continue;
          port = "found";
          const done = [];
          (cur.columns || []).forEach((col, k) => {
            const raw = (row.values || [])[k];
            if (raw == null || String(raw).trim() === "") return;
            const v = render(raw);
            r.set.push({ field: col, value: v });
            done.push(col + "=" + v);
          });
          step.detail = `${cur.field} = ${showValues(vals)} → linha "${row.match}": ${done.join("; ")}`;
          break;
        }
      }
      else if (cur.type === "update") {
        const u = { node: cur.id, fields: [], note: render(cur.note || ""), teams: !!cur.teams, repeat: !!cur.repeat };
        const done = [];
        (cur.set || []).forEach((a) => {
          const v = render(a.value);
          u.fields.push({ field: a.field, value: v });
          done.push(a.field + "=" + v);
        });
        if (u.note !== "") done.push("nota");
        if (u.teams) done.push("Teams");
        r.updates.push(u);
        step.detail = "atualizar chamado: " + done.join("; ");
        port = "next";
      }
      else if (cur.type === "zabbix") {
        const op = { node: cur.id, message: render(cur.message || ""), ack: !!cur.ack, severity: cur.severity ? severityLevel(cur.severity) : -1, suppress_hours: +cur.suppress_hours || 0, repeat: !!cur.repeat };
        r.zabbix.push(op);
        const done = [];
        if (op.message !== "") done.push("comentário");
        if (op.ack) done.push("ack");
        if (op.severity >= 0) done.push("severidade " + op.severity);
        if (op.suppress_hours > 0) done.push("suprimir " + op.suppress_hours + "h");
        step.detail = "Zabbix: " + done.join(", ");
        port = "next";
      }
      else if (cur.type === "note") {
        const note = { node: cur.id, text: render(cur.text || ""), target: cur.target || "internal", repeat: !!cur.repeat };
        r.notes.push(note);
        step.detail = "chamado (" + note.target + "): " + note.text;
        port = "next";
      }
      else if (cur.type === "http") {
        const call = { node: cur.id, method: cur.method || "POST", url: render(cur.url || ""), headers: (cur.headers || []).map(render), body: render(cur.body || ""), timeout: cur.timeout || 0 };
        r.http.push(call);
        step.detail = call.method + " " + call.url;
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
      case "start": case "set": case "update": case "http": case "transform": case "zabbix": case "note": return [{ port: "next", label: "" }];
      case "branch": return (n.branches || []).map((b) => ({ port: b.id, label: b.label || b.id })).concat([{ port: "else", label: "senão" }]);
      case "if": return [{ port: "true", label: "sim" }, { port: "false", label: "não" }];
      case "table": return [{ port: "found", label: "achou" }, { port: "not_found", label: "não achou" }];
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
    if (type === "branch") {
      n.branches = [
        { id: "r1", label: "Crítico", match: "all", conditions: [{ field: "severity.level", op: "gte", value: "4" }] },
        { id: "r2", label: "Médio", match: "all", conditions: [{ field: "severity.level", op: "eq", value: "3" }] },
      ];
    }
    if (type === "zabbix") { n.message = "goDesk: {trigger} — tratado pelo fluxo"; n.ack = false; n.severity = ""; n.suppress_hours = 0; n.repeat = false; }
    if (type === "note") { n.text = ""; n.target = "internal"; n.repeat = false; }
    if (type === "transform") { n.transforms = [{ var: "interface", from: "{trigger}", op: "extract", arg: "interface (\\S+)", arg2: "" }]; }
    if (type === "table") { n.field = "host.group"; n.columns = ["oper_group"]; n.rows = [{ match: "GRUPO/*", values: [""] }]; }
    if (type === "update") { n.set = [{ field: "urgency", value: "" }]; n.note = ""; n.teams = false; n.repeat = false; }
    if (type === "http") { n.method = "POST"; n.url = "https://"; n.headers = []; n.body = ""; n.timeout = 10; }
    return n;
  }

  function newFlow(name) {
    return { name, description: "", nodes: [{ id: "inicio", type: "start", label: "", x: 420, y: 30 }], links: [] };
  }

  // regras (com grupos) do arquivo → modelo do editor
  function prepConds(list) {
    return (list || []).map((c) => (c.conditions && c.conditions.length
      ? { match: c.match === "any" ? "any" : "all", conditions: prepConds(c.conditions) }
      : { field: String(c.field || ""), op: String(c.op || "eq"), value: String(c.value == null ? "" : c.value) }));
  }

  function cleanConds(list) {
    return list.map((c) => (isGroup(c)
      ? { match: c.match, conditions: cleanConds(c.conditions) }
      : { field: c.field.trim(), op: c.op, value: NOVALUE_OPS.has(c.op) ? "" : c.value }));
  }

  function isGroup(c) { return !!(c && c.conditions); }

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
        if (node.type === "if") { node.match = n.match === "any" ? "any" : "all"; node.conditions = prepConds(n.conditions); }
        if (node.type === "branch") {
          node.branches = (n.branches || []).map((b) => ({ id: String(b.id || ""), label: String(b.label || ""), match: b.match === "any" ? "any" : "all", conditions: prepConds(b.conditions) }));
        }
        if (node.type === "zabbix") {
          node.message = String(n.message || ""); node.ack = !!n.ack; node.severity = n.severity == null ? "" : String(n.severity);
          node.suppress_hours = +n.suppress_hours || 0; node.repeat = !!n.repeat;
        }
        if (node.type === "note") { node.text = String(n.text || ""); node.target = n.target || "internal"; node.repeat = !!n.repeat; }
        if (node.type === "transform") {
          node.transforms = (n.transforms || []).map((t) => ({ var: String(t.var || ""), from: String(t.from == null ? "" : t.from), op: String(t.op || "set"), arg: String(t.arg == null ? "" : t.arg), arg2: String(t.arg2 == null ? "" : t.arg2) }));
        }
        if (node.type === "switch") { node.field = String(n.field || ""); node.cases = (n.cases || []).map((c) => ({ value: String(c.value == null ? "" : c.value) })); }
        if (node.type === "set") { node.set = (n.set || []).map((a) => ({ field: String(a.field || ""), value: String(a.value == null ? "" : a.value) })); }
        if (node.type === "end") { node.action = n.action === "skip" ? "skip" : "open"; node.comment = String(n.comment || ""); }
        if (node.type === "table") {
          node.field = String(n.field || "");
          node.columns = (n.columns || []).map(String);
          node.rows = (n.rows || []).map((row) => ({ match: String(row.match == null ? "" : row.match), values: (row.values || []).map((v) => String(v == null ? "" : v)) }));
        }
        if (node.type === "update") {
          node.set = (n.set || []).map((a) => ({ field: String(a.field || ""), value: String(a.value == null ? "" : a.value) }));
          node.note = String(n.note || ""); node.teams = !!n.teams; node.repeat = !!n.repeat;
        }
        if (node.type === "http") {
          node.method = String(n.method || "POST").toUpperCase(); node.url = String(n.url || "");
          node.headers = (n.headers || []).map(String); node.body = String(n.body || ""); node.timeout = +n.timeout || 0;
        }
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
          if (n.type === "if") { o.match = n.match; o.conditions = cleanConds(n.conditions); }
          if (n.type === "branch") { o.branches = n.branches.map((b) => ({ id: b.id, label: b.label.trim(), match: b.match, conditions: cleanConds(b.conditions) })); }
          if (n.type === "zabbix") { o.message = n.message; o.ack = n.ack; o.severity = n.severity; o.suppress_hours = Math.max(0, Math.min(720, +n.suppress_hours || 0)); o.repeat = n.repeat; }
          if (n.type === "note") { o.text = n.text; o.target = n.target; o.repeat = n.repeat; }
          if (n.type === "transform") { o.transforms = n.transforms.map((t) => ({ var: t.var.trim(), from: t.from, op: t.op, arg: t.arg, arg2: t.arg2 })); }
          if (n.type === "switch") { o.field = n.field.trim(); o.cases = n.cases.map((c) => ({ value: c.value.trim() })); }
          if (n.type === "set") { o.set = n.set.map((a) => ({ field: a.field, value: a.value })); }
          if (n.type === "end") { o.action = n.action; o.comment = n.action === "skip" ? n.comment : ""; }
          if (n.type === "table") {
            o.field = n.field.trim(); o.columns = n.columns.slice();
            o.rows = n.rows.map((row) => {
              const values = n.columns.map((c, k) => row.values[k] == null ? "" : row.values[k]);
              while (values.length && values[values.length - 1] === "") values.pop();
              return { match: row.match.trim(), values };
            });
          }
          if (n.type === "update") { o.set = n.set.map((a) => ({ field: a.field, value: a.value })); o.note = n.note; o.teams = n.teams; o.repeat = n.repeat; }
          if (n.type === "http") {
            o.method = n.method; o.url = n.url.trim(); o.headers = n.headers.map((h) => h.trim()).filter(Boolean);
            o.body = n.body; o.timeout = Math.max(0, Math.min(30, +n.timeout || 0));
          }
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
    const tplRefs = (s) => (String(s).match(/\{([^{}]+)\}/g) || []).map((m) => {
      const [f, filter] = splitVar(m.slice(1, -1));
      if (filter && filter !== "json" && filter !== "url") return `filtro |${filter} inválido (use |json ou |url)`;
      return calRef(f);
    }).filter(Boolean);

    S.data.flows.forEach((f, fi) => {
      const fname = f.name.trim() || `#${fi + 1}`;
      if (!f.name.trim()) errs.push(`Fluxo #${fi + 1}: informe o nome.`);
      else if (names.has(lc(f.name))) errs.push(`Fluxo ${f.name} repetido.`);
      names.add(lc(f.name));
      f.nodes.forEach((n) => {
        const who = `Fluxo ${fname}, bloco ${n.label || n.id}`;
        const checkConds = (where, list, depth) => {
          if (!list.length) errs.push(`${where}: adicione pelo menos uma regra.`);
          list.forEach((c, i) => {
            if (isGroup(c)) {
              if (depth >= MAX_COND_DEPTH) errs.push(`${where}: grupos aninhados demais (máx ${MAX_COND_DEPTH} níveis).`);
              checkConds(`${where}, grupo #${i + 1}`, c.conditions, depth + 1);
              return;
            }
            if (!c.field.trim()) errs.push(`${where}: regra #${i + 1} sem campo.`);
            const e = calRef(c.field); if (e) errs.push(`${where}: ${e}.`);
            if (String(c.value).indexOf("{") < 0) {
              if ((c.op === "regex" || c.op === "not_regex") && !flowRegex(c.value)) errs.push(`${where}: regex inválida (${c.value}).`);
              if (c.op === "between" && !betweenParts(c.value)) errs.push(`${where}: "entre" espera início..fim (ex: 30..60 ou 22:00..06:00).`);
            }
            tplRefs(c.value).forEach((e2) => errs.push(`${where}: ${e2}.`));
          });
        };
        if (n.type === "if") checkConds(who, n.conditions, 1);
        if (n.type === "branch") {
          if (!n.branches.length) errs.push(`${who}: adicione pelo menos um ramo.`);
          const ids = new Set(["else", "next"]);
          n.branches.forEach((b, i) => {
            if (!b.id || ids.has(lc(b.id))) errs.push(`${who}: ramo #${i + 1} com id inválido ou repetido.`);
            ids.add(lc(b.id));
            checkConds(`${who}, ramo ${b.label || b.id}`, b.conditions, 1);
          });
        }
        if (n.type === "zabbix") {
          if (!n.message.trim() && !n.ack && !n.severity && !(+n.suppress_hours)) errs.push(`${who}: escolha pelo menos uma ação (comentário, ack, severidade ou suprimir).`);
          if (+n.suppress_hours < 0 || +n.suppress_hours > 720) errs.push(`${who}: suprimir entre 1 e 720 horas.`);
          tplRefs(n.message).forEach((e2) => errs.push(`${who}: ${e2}.`));
        }
        if (n.type === "note") {
          if (!n.text.trim()) errs.push(`${who}: escreva o texto.`);
          tplRefs(n.text).forEach((e2) => errs.push(`${who}: ${e2}.`));
        }
        if (n.type === "transform") {
          if (!n.transforms.length) errs.push(`${who}: adicione pelo menos uma operação.`);
          n.transforms.forEach((t, i) => {
            if (!/^[A-Za-z0-9_]+$/.test(t.var.trim())) errs.push(`${who}: operação #${i + 1}, nome da variável só com letras, números e _.`);
            if (t.op === "extract" && (!t.arg.trim() || !flowRegex(t.arg))) errs.push(`${who}: operação #${i + 1}, regex inválida.`);
            if (t.op === "replace" && t.arg === "") errs.push(`${who}: operação #${i + 1}, informe o texto a procurar.`);
            [t.from, t.arg2].concat(t.op === "default" ? [t.arg] : []).forEach((s) => tplRefs(s).forEach((e2) => errs.push(`${who}: ${e2}.`)));
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
        if (n.type === "table") {
          if (!n.field.trim()) errs.push(`${who}: informe o campo de busca.`);
          const e = calRef(n.field); if (e) errs.push(`${who}: ${e}.`);
          if (!n.columns.length) errs.push(`${who}: adicione pelo menos uma coluna.`);
          if (new Set(n.columns).size !== n.columns.length) errs.push(`${who}: coluna repetida.`);
          if (!n.rows.length) errs.push(`${who}: adicione pelo menos uma linha.`);
          n.rows.forEach((row, i) => {
            if (!row.match.trim()) errs.push(`${who}: linha #${i + 1} sem valor de busca.`);
            n.columns.forEach((c, k) => {
              const v = row.values[k] == null ? "" : row.values[k];
              if (BOOL_FIELDS.has(c) && v.trim() !== "" && !parseBool(v)[1]) errs.push(`${who}: linha #${i + 1}, ${c} precisa ser sim/não.`);
              tplRefs(v).forEach((e2) => errs.push(`${who}: ${e2}.`));
            });
          });
        }
        if (n.type === "update") {
          if (!n.set.length && !n.note.trim() && !n.teams) errs.push(`${who}: defina um campo, uma nota ou o Teams.`);
          n.set.forEach((a) => { tplRefs(a.value).forEach((e2) => errs.push(`${who}: ${e2}.`)); });
          tplRefs(n.note).forEach((e2) => errs.push(`${who}: ${e2}.`));
        }
        if (n.type === "http") {
          if (!/^https?:\/\//i.test(n.url.trim())) errs.push(`${who}: a URL precisa começar com http:// ou https://.`);
          if (+n.timeout < 0 || +n.timeout > 30) errs.push(`${who}: timeout entre 1 e 30 segundos.`);
          n.headers.filter((h) => h.trim()).forEach((h, i) => {
            const k = h.indexOf(":");
            if (k <= 0 || !h.slice(0, k).trim()) errs.push(`${who}: cabeçalho #${i + 1} no formato "Nome: valor".`);
          });
          [n.url, n.body].concat(n.headers).forEach((s) => tplRefs(s).forEach((e2) => errs.push(`${who}: ${e2}.`)));
        }
      });
    });
    return errs;
  }

  // =========================================================
  // layout
  // =========================================================

  function nodeWidth(n) { return n.type === "switch" || n.type === "branch" ? Math.max(NODE_W, ports(n).length * 76) : NODE_W; }

  function summary(fl, n) {
    const w = Math.floor(nodeWidth(n) / 6.6);
    switch (n.type) {
      case "start": return ["Alerta recebido"];
      case "if": {
        const join = n.match === "any" ? "OU " : "E ";
        const lines = n.conditions.slice(0, 3).map((c, i) => trunc((i ? join : "") + condText(c), w));
        if (n.conditions.length > 3) lines.push(`+${n.conditions.length - 3} regra(s)`);
        return lines.length ? lines : ["(sem regras)"];
      }
      case "branch": {
        const lines = n.branches.slice(0, 3).map((b, i) => trunc(`${i ? "senão se" : "se"} ${b.label || b.id}: ${b.conditions.map(condText).join(b.match === "any" ? " OU " : " E ")}`, w));
        if (n.branches.length > 3) lines.push(`+${n.branches.length - 3} ramo(s)`);
        return lines.length ? lines : ["(sem ramos)"];
      }
      case "zabbix": {
        const lines = [];
        if (n.message) lines.push(trunc("💬 " + n.message, w));
        const acts = [n.ack ? "ack" : "", n.severity !== "" ? "sev. " + ((SEVERITIES.find(([k]) => k === String(severityLevel(n.severity))) || ["", n.severity])[1]) : "", +n.suppress_hours ? "suprime " + n.suppress_hours + "h" : ""].filter(Boolean);
        if (acts.length) lines.push(trunc(acts.join(" · "), w));
        lines.push(n.repeat ? "a cada reenvio" : "1x por evento");
        return lines;
      }
      case "note": {
        const where = { internal: "interna", public: "visível", description: "descrição" }[n.target] || n.target;
        return [trunc(`[${where}] ${n.text || "(vazio)"}`, w)];
      }
      case "transform": {
        const lines = n.transforms.slice(0, 3).map((t) => trunc(`var.${t.var} ← ${(TRANSFORM_OPS.find(([k]) => k === t.op) || [t.op, t.op])[1]}`, w));
        if (n.transforms.length > 3) lines.push(`+${n.transforms.length - 3} operação(ões)`);
        return lines.length ? lines : ["(nenhuma operação)"];
      }
      case "switch": return [trunc("valor de " + (n.field || "?"), w)];
      case "set": {
        const lines = n.set.slice(0, 3).map((a) => trunc(`${a.field} = ${a.value}`, w));
        if (n.set.length > 3) lines.push(`+${n.set.length - 3} campo(s)`);
        return lines.length ? lines : ["(nenhum campo)"];
      }
      case "end": return [n.action === "skip" ? "Não abrir chamado" : "Abrir chamado"].concat(n.action === "skip" && n.comment ? [trunc(n.comment, w)] : []);
      case "table": {
        const lines = [trunc(`${n.field || "?"} → ${n.columns.join(", ")}`, w)];
        n.rows.slice(0, 2).forEach((row) => lines.push(trunc(`${row.match}: ${row.values.filter((v) => v !== "").join(", ")}`, w)));
        if (n.rows.length > 2) lines.push(`+${n.rows.length - 2} linha(s)`);
        return lines;
      }
      case "update": {
        const lines = n.set.slice(0, 2).map((a) => trunc(`${a.field} = ${a.value}`, w));
        if (n.note) lines.push(trunc("nota: " + n.note, w));
        const flags = [n.teams ? "Teams" : "", n.repeat ? "a cada reenvio" : "1x por evento"].filter(Boolean).join(" · ");
        lines.push(flags);
        return lines;
      }
      case "http": return [trunc(`${n.method} ${n.url}`, w)];
    }
    return [];
  }

  // texto curto de uma regra (grupo vira "(… OU …)")
  function condText(c) {
    if (isGroup(c)) return "(" + c.conditions.map(condText).join(c.match === "any" ? " OU " : " E ") + ")";
    return `${c.field} ${OP_LABEL[c.op] || c.op}${NOVALUE_OPS.has(c.op) ? "" : " " + c.value}`;
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
    const desc = $("gd-flow-desc");
    if (desc) { const f = flow(); desc.textContent = f ? f.description || "—" : ""; }
    const box = $("gd-flow-rules");
    if (!box) return;
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
      box.innerHTML = S.readonly
        ? '<div class="gd-muted">Nenhum calendário cadastrado.</div>'
        : '<div class="gd-muted">Nenhum calendário. Crie um (ex: <b>comercial</b>, seg a sex 08:00–18:00) para usar “fora do horário comercial” nas condições.</div>';
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
    if (S.readonly) lockSide(box);
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

  // ---------- editor de regras com grupos ----------
  // chave de uma lista de regras: "if" (bloco condição), "br:<i>" (ramo da
  // condição múltipla) e "<chave>/<j>" (grupo j dentro da lista).

  function condOwner(n, key) {
    const parts = key.split("/");
    let owner = parts[0] === "if" ? n : n.branches[+parts[0].slice(3)];
    for (const j of parts.slice(1)) owner = owner.conditions[+j];
    return owner;
  }

  function matchSelect(key, match) {
    return `<select data-gd-cl-match="${escapeHtml(key)}">
      <option value="all"${match !== "any" ? " selected" : ""}>todas (E)</option>
      <option value="any"${match === "any" ? " selected" : ""}>qualquer (OU)</option></select>`;
  }

  function condsHtml(n, key, depth) {
    const owner = condOwner(n, key);
    let html = "";
    owner.conditions.forEach((c, j) => {
      const ref = escapeHtml(key + "|" + j);
      if (isGroup(c)) {
        html += `<div class="gd-flow-group">
          <div class="gd-flow-group-head">grupo: ${matchSelect(key + "/" + j, c.match)}
            <button type="button" class="gd-btn gd-btn-danger gd-btn-xs" data-gd-c-remove="${ref}">✕</button></div>
          ${condsHtml(n, key + "/" + j, depth + 1)}
        </div>`;
        return;
      }
      html += `<div class="gd-flow-rule">
        <input type="text" list="gd-flow-fields" autocomplete="off" data-gd-c="${ref}" data-k="field" value="${escapeHtml(c.field)}" placeholder="campo">
        <select data-gd-c="${ref}" data-k="op">${opOptions(c.op)}</select>
        <input type="text" data-gd-c="${ref}" data-k="value" value="${escapeHtml(c.value)}" placeholder="${c.op === "between" ? "início..fim" : "valor"}"${NOVALUE_OPS.has(c.op) ? " disabled" : ""}>
        <button type="button" class="gd-btn gd-btn-danger gd-btn-xs" data-gd-c-remove="${ref}">✕</button></div>`;
    });
    html += `<div class="gd-actions gd-flow-cond-actions"><button type="button" class="gd-btn gd-btn-xs" data-gd-c-add="${escapeHtml(key)}">+ Regra</button>` +
      (depth < MAX_COND_DEPTH ? `<button type="button" class="gd-btn gd-btn-xs" data-gd-g-add="${escapeHtml(key)}">+ Grupo (E/OU)</button>` : "") + "</div>";
    return html;
  }

  function condsHelp() {
    return '<div class="gd-muted">Campo com vários valores (grupos, tags repetidas): “igual/contém/corresponde” vale se <b>algum</b> bater; “diferente/não…” se <b>nenhum</b> bater. Lista: valores separados por vírgula, aceita <code>*</code>. <b>Grupo</b>: regras com o próprio E/OU — ex: (tag = Critico OU grupo começa com Core/) E fora do horário. <b>Entre</b>: <code>30..60</code> ou <code>22:00..06:00</code> (atravessa a meia-noite). <b>Regex</b> sem diferenciar maiúsculas.</div>';
  }

  function newBranchId(n) {
    let i = 1;
    while (n.branches.some((b) => lc(b.id) === "r" + i)) i++;
    return "r" + i;
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
      if (S.readonly) lockSide(box);
      return;
    }

    const n = S.sel && S.sel.type === "node" ? nodeOf(fl, S.sel.id) : null;
    if (!n && S.readonly) {
      box.innerHTML = `<div class="gd-small-title">${escapeHtml(fl.name)}</div>
        ${fl.description ? `<div style="white-space:pre-wrap">${escapeHtml(fl.description)}</div>` : '<div class="gd-muted">Sem descrição.</div>'}
        <div class="gd-muted" style="margin-top:10px;">Clique num bloco para ver os detalhes. Com o 🧪 Testar aberto, o caminho do alerta fica destacado em verde.</div>`;
      return;
    }
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
      html += `<div class="gd-field" style="margin-top:8px;"><label>Vale quando</label>${matchSelect("if", n.match)}</div>`;
      html += '<div class="gd-small-title" style="margin-top:10px;">Regras</div>';
      html += condsHtml(n, "if", 1);
      html += condsHelp();
    }

    if (n.type === "branch") {
      html += '<div class="gd-muted" style="margin-top:6px;">Os ramos são testados <b>em ordem</b>: o primeiro que valer escolhe a saída; nenhum = <b>senão</b>.</div>';
      n.branches.forEach((b, i) => {
        html += `<div class="gd-flow-branch">
          <div class="gd-flow-branch-head"><b>${i ? "senão se" : "se"}</b>
            <input type="text" data-gd-br="${i}" data-k="label" value="${escapeHtml(b.label)}" placeholder="${escapeHtml(b.id)}" title="nome da saída">
            <button type="button" class="gd-btn gd-btn-xs" data-gd-br-move="${i}:-1" title="subir">↑</button>
            <button type="button" class="gd-btn gd-btn-xs" data-gd-br-move="${i}:1" title="descer">↓</button>
            <button type="button" class="gd-btn gd-btn-danger gd-btn-xs" data-gd-br-remove="${i}">✕</button></div>
          <div class="gd-field"><label>Vale quando</label>${matchSelect("br:" + i, b.match)}</div>
          ${condsHtml(n, "br:" + i, 1)}
        </div>`;
      });
      html += '<div class="gd-flow-branch gd-flow-branch-else"><b>senão</b> <span class="gd-muted">— nenhum ramo valeu (saída “senão”)</span></div>';
      html += '<div class="gd-actions"><button type="button" class="gd-btn" data-gd-flow="br-add">+ Ramo (senão se)</button></div>';
      html += condsHelp();
    }

    if (n.type === "zabbix") {
      html += `<div class="gd-field" style="margin-top:8px;"><label>Comentário no evento <span class="gd-muted">(vazio = sem comentário)</span></label><textarea rows="2" data-gd-node-prop="message" placeholder="ex: Analisado pelo fluxo: {var.interface} — plantão acionado">${escapeHtml(n.message)}</textarea></div>`;
      html += `<label class="gd-check" style="margin-top:8px;"><input type="checkbox" data-gd-node-check="ack"${n.ack ? " checked" : ""}> <span>Reconhecer o evento (ack)</span></label>`;
      html += `<div class="gd-row" style="margin-top:8px;"><div class="gd-field gd-field-tight"><label>Mudar severidade</label><select data-gd-node-prop="severity">${SEVERITIES.map(([v, l]) => `<option value="${v}"${String(n.severity === "" ? "" : severityLevel(n.severity)) === v ? " selected" : ""}>${escapeHtml(l)}</option>`).join("")}</select></div>
        <div class="gd-field gd-field-tight" style="min-width:110px;"><label>Suprimir por (h)</label><input type="number" min="0" max="720" data-gd-node-prop="suppress_hours" value="${escapeHtml(n.suppress_hours || 0)}"></div></div>`;
      html += `<label class="gd-check" style="margin-top:6px;"><input type="checkbox" data-gd-node-check="repeat"${n.repeat ? " checked" : ""}> <span>Repetir a cada reenvio <span class="gd-muted">(padrão: uma vez por evento)</span></span></label>`;
      html += varChips();
      html += '<div class="gd-muted">Executa no fim do alerta (só no modo ativo). O ack feito aqui é do usuário do goDesk e <b>não</b> conta como “reconhecido pelo atendimento” (<code>event.acked_by_user</code>) — o goDesk se reconhece pelas mensagens que começam com “Chamado …” ou “goDesk …”; num fluxo que não abre chamado, comece o comentário com “goDesk”. Mudar severidade e suprimir dependem da permissão do usuário do token (suprimir: Zabbix 6.2+).</div>';
    }

    if (n.type === "note") {
      html += `<div class="gd-field" style="margin-top:8px;"><label>Onde</label><select data-gd-node-prop="target">${NOTE_TARGETS.map(([v, l]) => `<option value="${v}"${v === n.target ? " selected" : ""}>${escapeHtml(l)}</option>`).join("")}</select></div>`;
      html += `<div class="gd-field" style="margin-top:8px;"><label>Texto</label><textarea rows="4" data-gd-node-prop="text" placeholder="ex: Procedimento: verificar o link da operadora {host.tag.Operadora} antes de acionar o cliente.">${escapeHtml(n.text)}</textarea></div>`;
      if (n.target !== "description") {
        html += `<label class="gd-check" style="margin-top:6px;"><input type="checkbox" data-gd-node-check="repeat"${n.repeat ? " checked" : ""}> <span>Repetir a cada reenvio <span class="gd-muted">(padrão: uma vez por evento)</span></span></label>`;
      }
      html += varChips();
      html += '<div class="gd-muted"><b>Descrição</b>: entra no texto de abertura do chamado (o solicitante vê). <b>Ação interna/visível</b>: registrada logo depois de abrir e, se o alerta voltar com o chamado aberto, uma vez por evento (ou a cada reenvio, se marcado).</div>';
    }

    if (n.type === "transform") {
      html += '<div class="gd-muted" style="margin-top:6px;">Cada operação cria a variável <code>{var.nome}</code>, que pode ser usada nos blocos seguintes (condições e textos) — inclusive pelas operações abaixo dela.</div>';
      n.transforms.forEach((t, i) => {
        const needArg = t.op === "extract" || t.op === "replace" || t.op === "default";
        const argPh = t.op === "extract" ? "regex — ex: interface (\\S+)" : t.op === "replace" ? "procurar" : "valor se vazio";
        html += `<div class="gd-flow-branch">
          <div class="gd-flow-rule"><span class="gd-muted">var.</span><input type="text" data-gd-tf="${i}" data-k="var" value="${escapeHtml(t.var)}" placeholder="nome">
            <select data-gd-tf="${i}" data-k="op">${TRANSFORM_OPS.map(([v, l]) => `<option value="${v}"${v === t.op ? " selected" : ""}>${escapeHtml(l)}</option>`).join("")}</select>
            <button type="button" class="gd-btn gd-btn-danger gd-btn-xs" data-gd-tf-remove="${i}">✕</button></div>
          <div class="gd-flow-rule"><input type="text" data-gd-tf="${i}" data-k="from" value="${escapeHtml(t.from)}" placeholder="de — ex: {trigger}"></div>
          ${needArg ? `<div class="gd-flow-rule"><input type="text" data-gd-tf="${i}" data-k="arg" value="${escapeHtml(t.arg)}" placeholder="${escapeHtml(argPh)}">${t.op === "replace" ? `<input type="text" data-gd-tf="${i}" data-k="arg2" value="${escapeHtml(t.arg2)}" placeholder="trocar por">` : ""}</div>` : ""}
        </div>`;
      });
      html += '<div class="gd-actions"><button type="button" class="gd-btn" data-gd-flow="tf-add">+ Operação</button></div>';
      html += varChips();
      html += '<div class="gd-muted"><b>Extrair por regex</b>: pega o 1º grupo entre parênteses (ou o trecho todo), sem diferenciar maiúsculas — ex: de <code>{trigger}</code> com <code>interface (\\S+)</code> sai <code>Gi0/1</code>. <b>Substituir</b> troca todas as ocorrências do texto. <b>Valor padrão</b> só entra quando a origem vem vazia.</div>';
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

    if (n.type === "table") {
      html += `<div class="gd-field" style="margin-top:8px;"><label>Buscar pelo campo</label><input type="text" list="gd-flow-fields" autocomplete="off" data-gd-node-prop="field" value="${escapeHtml(n.field)}" placeholder="host.group"></div>`;
      html += '<div class="gd-small-title" style="margin-top:10px;">Colunas <span class="gd-muted">(o que cada linha define)</span></div>';
      n.columns.forEach((c, k) => {
        html += `<div class="gd-flow-rule"><select data-gd-tcol="${k}">${settableOptions(c)}</select>
          <button type="button" class="gd-btn gd-btn-danger gd-btn-xs" data-gd-tcol-remove="${k}">✕</button></div>`;
      });
      html += '<div class="gd-actions"><button type="button" class="gd-btn" data-gd-flow="tcol-add">+ Coluna</button></div>';
      html += '<div class="gd-small-title" style="margin-top:10px;">Linhas <span class="gd-muted">(a primeira que bater vale)</span></div>';
      html += '<div class="gd-flow-table"><table><thead><tr><th>valor</th>' + n.columns.map((c) => `<th>${escapeHtml(trunc(SETTABLE_LABEL[c] || c, 18))}</th>`).join("") + '<th></th></tr></thead><tbody>';
      n.rows.forEach((row, i) => {
        html += `<tr><td><input type="text" data-gd-trow="${i}" data-k="match" value="${escapeHtml(row.match)}" placeholder="ex: POWERPLANT/*"></td>`;
        n.columns.forEach((c, k) => {
          const v = row.values[k] == null ? "" : row.values[k];
          html += BOOL_FIELDS.has(c)
            ? `<td><select data-gd-trow="${i}" data-col="${k}"><option value=""${v === "" ? " selected" : ""}>—</option><option value="true"${v !== "" && parseBool(v)[0] ? " selected" : ""}>sim</option><option value="false"${v !== "" && !parseBool(v)[0] ? " selected" : ""}>não</option></select></td>`
            : `<td><input type="text" data-gd-trow="${i}" data-col="${k}" value="${escapeHtml(v)}"></td>`;
        });
        html += `<td><button type="button" class="gd-btn gd-btn-danger gd-btn-xs" data-gd-trow-remove="${i}">✕</button></td></tr>`;
      });
      html += '</tbody></table></div>';
      html += '<div class="gd-actions"><button type="button" class="gd-btn" data-gd-flow="trow-add">+ Linha</button></div>';
      html += varChips();
      html += '<div class="gd-muted">Valor da linha: lista separada por vírgula, aceita <code>*</code>, sem diferenciar maiúsculas. Célula vazia não define nada. Saída <b>achou</b> / <b>não achou</b>.</div>';
    }

    if (n.type === "update") {
      html += '<div class="gd-banner gd-warn" style="margin-top:8px;">Só age quando o alerta <b>volta</b> (reenvio do escalonamento da action) e o chamado <b>já existe</b>. Na abertura, este bloco é ignorado.</div>';
      html += '<div class="gd-small-title" style="margin-top:10px;">Mudar no chamado</div>';
      n.set.forEach((a, i) => {
        html += `<div class="gd-flow-rule gd-flow-setrow"><select data-gd-upd="${i}" data-k="field">${UPDATE_FIELDS.map(([v, l]) => `<option value="${v}"${v === a.field ? " selected" : ""}>${escapeHtml(l)}</option>`).join("")}</select>
          <input type="text" data-gd-upd="${i}" data-k="value" value="${escapeHtml(a.value)}">
          <button type="button" class="gd-btn gd-btn-danger gd-btn-xs" data-gd-upd-remove="${i}">✕</button></div>`;
      });
      html += '<div class="gd-actions"><button type="button" class="gd-btn" data-gd-flow="upd-add">+ Campo</button></div>';
      html += `<div class="gd-field" style="margin-top:8px;"><label>Nota no chamado <span class="gd-muted">(invisível pro solicitante)</span></label><textarea rows="2" data-gd-node-prop="note" placeholder="ex: Sem reconhecimento há {event.age} min — escalado para N2">${escapeHtml(n.note)}</textarea></div>`;
      html += `<label class="gd-check" style="margin-top:8px;"><input type="checkbox" data-gd-node-check="teams"${n.teams ? " checked" : ""}> <span>Avisar no Teams (webhook da rule/fluxo)</span></label>`;
      html += `<label class="gd-check" style="margin-top:4px;"><input type="checkbox" data-gd-node-check="repeat"${n.repeat ? " checked" : ""}> <span>Repetir a cada reenvio <span class="gd-muted">(padrão: uma vez por evento)</span></span></label>`;
      html += varChips();
      html += '<div class="gd-muted">Exemplo: Condição <code>event.age ≥ 30</code> E <code>event.acked_by_user é falso</code> → Atualizar chamado (urgência Crítica, grupo N2, Teams). Na action do Zabbix, configure passos que reenviam a cada X minutos.</div>';
    }

    if (n.type === "http") {
      html += `<div class="gd-row" style="margin-top:8px;"><div class="gd-field gd-field-tight" style="min-width:90px;"><label>Método</label><select data-gd-node-prop="method">${HTTP_METHODS.map((m) => `<option${m === n.method ? " selected" : ""}>${m}</option>`).join("")}</select></div>
        <div class="gd-field gd-field-tight" style="min-width:80px;"><label>Timeout (s)</label><input type="number" min="1" max="30" data-gd-node-prop="timeout" value="${escapeHtml(n.timeout || 10)}"></div></div>`;
      html += `<div class="gd-field" style="margin-top:8px;"><label>URL</label><input type="text" data-gd-node-prop="url" value="${escapeHtml(n.url)}" placeholder="https://api.exemplo/hook?host={host|url}"></div>`;
      html += `<div class="gd-field" style="margin-top:8px;"><label>Cabeçalhos <span class="gd-muted">(Nome: valor, um por linha)</span></label><textarea rows="2" data-gd-node-lines="headers" placeholder="Authorization: Bearer ...">${escapeHtml(n.headers.join("\n"))}</textarea></div>`;
      html += `<div class="gd-field" style="margin-top:8px;"><label>Corpo</label><textarea rows="4" data-gd-node-prop="body" placeholder='{"chamado": "{ticket.number}", "host": "{host|json}"}'>${escapeHtml(n.body)}</textarea></div>`;
      html += varChips();
      html += '<div class="gd-muted">Executa <b>depois</b> que o goDesk trata o chamado (só no modo ativo, não em shadow) e não muda o caminho do fluxo. <code>{ticket.number}</code> vira o número do chamado. Use <code>{campo|json}</code> dentro de JSON e <code>{campo|url}</code> na URL. Com corpo e sem Content-Type, vai <code>application/json</code>. O resultado aparece no Histórico.</div>';
    }

    if (n.type !== "start") {
      html += '<div class="gd-actions gd-flow-edit-only"><button type="button" class="gd-btn gd-btn-danger" data-gd-flow="node-remove">Remover bloco</button></div>';
    }
    box.innerHTML = html;
    if (S.readonly) lockSide(box);
  }

  // somente leitura: o painel mostra os detalhes, mas nada é editável
  function lockSide(box) {
    box.querySelectorAll("input, select, textarea").forEach((e) => { e.disabled = true; });
    box.querySelectorAll("button").forEach((b) => b.remove());
    box.querySelectorAll(".gd-flow-vars, .gd-flow-edit-only").forEach((e) => e.remove());
  }

  // ---------- teste ----------

  function updatesHtml(r) {
    if (!r.updates || !r.updates.length) return "";
    const kind = testInput("event.kind") ? testInput("event.kind").value || "problem" : "problem";
    const exists = testInput("ticket.exists") && testInput("ticket.exists").value === "true" && kind === "problem";
    const items = r.updates.map((u) => {
      const parts = u.fields.filter((a) => a.value !== "").map((a) => `${(UPDATE_FIELDS.find(([k]) => k === a.field) || [a.field, a.field])[1]} = ${a.value}`);
      if (u.note) parts.push("nota: " + u.note);
      if (u.teams) parts.push("Teams");
      return `<li>${escapeHtml(parts.join(" · ") || "(nada)")} <span class="gd-muted">(${u.repeat ? "a cada reenvio" : "uma vez por evento"})</span></li>`;
    }).join("");
    return `<div class="gd-small-title" style="margin-top:10px;">Atualizar chamado</div>
      ${exists ? "" : '<div class="gd-muted">Só aconteceria num <b>reenvio</b> com o chamado já aberto (marque “Chamado já existe? = sim”).</div>'}
      <ul class="gd-flow-res">${items}</ul>`;
  }

  function notesHtml(r) {
    if (!r.notes || !r.notes.length) return "";
    const where = { internal: "ação interna", public: "ação visível", description: "descrição" };
    return `<div class="gd-small-title" style="margin-top:10px;">Texto no chamado</div>
      <ul class="gd-flow-res">${r.notes.map((n) => `<li><b>${escapeHtml(where[n.target] || n.target)}</b>: <span style="white-space:pre-wrap">${escapeHtml(n.text)}</span></li>`).join("")}</ul>`;
  }

  function zabbixHtml(r) {
    if (!r.zabbix || !r.zabbix.length) return "";
    return `<div class="gd-small-title" style="margin-top:10px;">Ação no Zabbix <span class="gd-muted">(no fim do alerta)</span></div>
      <ul class="gd-flow-res">${r.zabbix.map((z) => {
        const parts = [];
        if (z.message) parts.push("comentário: " + z.message);
        if (z.ack) parts.push("ack");
        if (z.severity >= 0) parts.push("severidade → " + SEVERITIES[z.severity + 1][1]);
        if (z.suppress_hours > 0) parts.push("suprimir " + z.suppress_hours + "h");
        return `<li>${escapeHtml(parts.join(" · "))} <span class="gd-muted">(${z.repeat ? "a cada reenvio" : "uma vez por evento"})</span></li>`;
      }).join("")}</ul>`;
  }

  function varsHtml(r) {
    const keys = Object.keys(r.vars || {});
    if (!keys.length) return "";
    return `<div class="gd-small-title" style="margin-top:10px;">Variáveis</div>
      <ul class="gd-flow-res">${keys.map((k) => `<li><code>{var.${escapeHtml(k)}}</code> = ${escapeHtml(r.vars[k] === "" ? "(vazio)" : r.vars[k])}</li>`).join("")}</ul>`;
  }

  function httpHtml(r) {
    if (!r.http || !r.http.length) return "";
    return `<div class="gd-small-title" style="margin-top:10px;">HTTP <span class="gd-muted">(depois de tratar o chamado; não é chamado no teste)</span></div>
      <ul class="gd-flow-res">${r.http.map((c) => `<li><b>${escapeHtml(c.method)}</b> ${escapeHtml(c.url)}${c.body ? `<br><code class="gd-flow-code">${escapeHtml(trunc(c.body, 300))}</code>` : ""}</li>`).join("")}</ul>`;
  }

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
    ["host", "host.name", "trigger", "severity", "event.kind", "ticket.exists", "rule",
      "event.age", "event.resends", "event.acked_by_user", "event.acknowledged"].forEach((k) => { if (String(v(k)).trim() !== "") add(k, v(k)); });
    const sev = ["not classified", "information", "warning", "average", "high", "disaster"].indexOf(lc(v("severity")));
    if (sev >= 0) add("severity.level", String(sev));
    v("host.group").split(/\r?\n/).map((s) => s.trim()).filter(Boolean).forEach((g) => add("host.group", g));
    parseKV(v("host_tags")).forEach(([t, val]) => add("host.tag." + t, val));
    parseKV(v("event_tags")).forEach(([t, val]) => { add("event.tag." + t, val); add("tag." + t, val); });
    parseKV(v("extra")).forEach(([f, val]) => add(f, val));
    return (field) => {
      const k = lc(field);
      if (k === "host.groups") return map["host.group"] || [];
      // como no goDesk: sem chamado ainda, o número fica pra depois
      if (k === "ticket.number" && !map[k]) return ["{ticket.number}"];
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
          <div class="gd-small-title" style="margin-top:10px;">Campos definidos pelo fluxo</div>${sets}
          ${updatesHtml(r)}${notesHtml(r)}${zabbixHtml(r)}${httpHtml(r)}${varsHtml(r)}</div>
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
    const port = S.readonly ? null : e.target.closest(".gd-flow-port-out");
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
    if (!S.drag || S.readonly) return;
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
    if (!fl || !S.sel || S.readonly) return;
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
    if (S.readonly) return;
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
    // switch não tem saída "next": ligação sem porta só pode ser de um caso
    // que ficou vazio enquanto era editado — ela acompanha o caso também
    fl.links.forEach((l) => { if (l.from === n.id && lc(l.port || "") === lc(old)) l.port = value; });
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
        set("event.age", ev["event.age"] || "0");
        set("event.acked_by_user", ev["event.acked_by_user"] || "false");
        set("event.acknowledged", ev["event.acknowledged"] || "false");
        set("now", ev.clock);
        set("host.group", (ev["host.group"] || []).join("\n"));
        set("host_tags", (ev.host_tags || []).map((t) => t.tag + "=" + t.value).join("\n"));
        set("event_tags", (ev.event_tags || []).map((t) => t.tag + "=" + t.value).join("\n"));
        // inventário, macros e problemas do host; item:<chave> e
        // group.problems.<grupo> o usuário digita aqui se o fluxo usar
        set("extra", (ev.extra || []).join("\n"));
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
      readonly: !!payload.readonly,
    };
    if (S.readonly) {
      S.testOpen = true;
      const nowIn = testInput("now");
      if (nowIn && !nowIn.value) nowIn.value = fmtDate(new Date()) + "T" + fmtTime(new Date());
    }

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
      const rm = e.target.closest("[data-gd-cond-remove],[data-gd-case-remove],[data-gd-set-remove],[data-gd-tcol-remove],[data-gd-trow-remove],[data-gd-upd-remove]");
      if (rm && n) {
        if (rm.dataset.gdTcolRemove !== undefined) {
          const k = +rm.dataset.gdTcolRemove;
          n.columns.splice(k, 1);
          n.rows.forEach((row) => row.values.splice(k, 1));
        }
        if (rm.dataset.gdTrowRemove !== undefined) n.rows.splice(+rm.dataset.gdTrowRemove, 1);
        if (rm.dataset.gdUpdRemove !== undefined) n.set.splice(+rm.dataset.gdUpdRemove, 1);
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

      // regras/grupos (condição e condição múltipla), ramos e transformar
      const lsAct = n && !S.readonly ? e.target.closest("[data-gd-c-remove],[data-gd-c-add],[data-gd-g-add],[data-gd-br-move],[data-gd-br-remove],[data-gd-tf-remove]") : null;
      if (lsAct) {
        const ds = lsAct.dataset;
        if (ds.gdCRemove !== undefined) {
          const [key, j] = ds.gdCRemove.split("|");
          condOwner(n, key).conditions.splice(+j, 1);
        }
        else if (ds.gdCAdd !== undefined) condOwner(n, ds.gdCAdd).conditions.push({ field: "", op: "eq", value: "" });
        else if (ds.gdGAdd !== undefined) condOwner(n, ds.gdGAdd).conditions.push({ match: "any", conditions: [{ field: "", op: "eq", value: "" }] });
        else if (ds.gdBrMove !== undefined) {
          const [i, d] = ds.gdBrMove.split(":").map(Number);
          const k = i + d;
          if (k >= 0 && k < n.branches.length) { const t = n.branches[i]; n.branches[i] = n.branches[k]; n.branches[k] = t; }
        }
        else if (ds.gdBrRemove !== undefined) {
          const b = n.branches[+ds.gdBrRemove];
          fl.links = fl.links.filter((l) => !(l.from === n.id && lc(l.port || "") === lc(b.id)));
          n.branches.splice(+ds.gdBrRemove, 1);
        }
        else if (ds.gdTfRemove !== undefined) n.transforms.splice(+ds.gdTfRemove, 1);
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
      else if (a === "br-add" && n) {
        n.branches.push({ id: newBranchId(n), label: "", match: "all", conditions: [{ field: "", op: "eq", value: "" }] });
        dirty(); renderCanvas(); renderSide();
      }
      else if (a === "tf-add" && n) {
        let k = n.transforms.length + 1;
        while (n.transforms.some((t) => lc(t.var) === "v" + k)) k++;
        n.transforms.push({ var: "v" + k, from: "", op: "set", arg: "", arg2: "" });
        dirty(); renderCanvas(); renderSide();
      }
      else if (a === "tcol-add" && n) {
        const used = new Set(n.columns);
        const c = (SETTABLE.find(([k]) => !used.has(k)) || [null])[0];
        if (c) { n.columns.push(c); n.rows.forEach((row) => { while (row.values.length < n.columns.length) row.values.push(""); }); dirty(); renderCanvas(); renderSide(); }
      }
      else if (a === "trow-add" && n) { n.rows.push({ match: "", values: n.columns.map(() => "") }); dirty(); renderCanvas(); renderSide(); }
      else if (a === "upd-add" && n) {
        const used = new Set(n.set.map((x) => x.field));
        const f = (UPDATE_FIELDS.find(([k]) => !used.has(k)) || ["urgency"])[0];
        n.set.push({ field: f, value: "" }); dirty(); renderCanvas(); renderSide();
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
      if (t.dataset.gdC !== undefined && n && t.tagName !== "SELECT") {
        const [key, j] = t.dataset.gdC.split("|");
        condOwner(n, key).conditions[+j][t.dataset.k] = t.value; dirty(); renderCanvas(); renderTest();
        return;
      }
      if (t.dataset.gdBr !== undefined && n) {
        n.branches[+t.dataset.gdBr][t.dataset.k] = t.value; dirty(); renderCanvas(); renderTest();
        return;
      }
      if (t.dataset.gdTf !== undefined && n && t.tagName !== "SELECT") {
        n.transforms[+t.dataset.gdTf][t.dataset.k] = t.value; dirty(); renderCanvas(); renderTest();
        return;
      }
      if (t.dataset.gdUpd !== undefined && n && t.tagName !== "SELECT") {
        n.set[+t.dataset.gdUpd][t.dataset.k] = t.value; dirty(); renderCanvas(); renderTest();
        return;
      }
      if (t.dataset.gdTrow !== undefined && n && t.tagName !== "SELECT") {
        const row = n.rows[+t.dataset.gdTrow];
        if (t.dataset.k === "match") row.match = t.value;
        else { while (row.values.length < n.columns.length) row.values.push(""); row.values[+t.dataset.col] = t.value; }
        dirty(); renderCanvas(); renderTest();
        return;
      }
      if (t.dataset.gdNodeLines && n) {
        n[t.dataset.gdNodeLines] = t.value.split(/\r?\n/);
        dirty(); renderCanvas(); renderTest();
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
      if (S.readonly && t.dataset.gdTest === undefined) return;
      if (t.dataset.gdC !== undefined && n && t.tagName === "SELECT") {
        const [key, j] = t.dataset.gdC.split("|");
        condOwner(n, key).conditions[+j][t.dataset.k] = t.value; dirty(); renderCanvas(); renderSide(); renderTest();
        return;
      }
      if (t.dataset.gdClMatch !== undefined && n) {
        condOwner(n, t.dataset.gdClMatch).match = t.value; dirty(); renderCanvas(); renderTest();
        return;
      }
      if (t.dataset.gdTf !== undefined && n && t.tagName === "SELECT") {
        n.transforms[+t.dataset.gdTf][t.dataset.k] = t.value; dirty(); renderCanvas(); renderSide(); renderTest();
        return;
      }
      if (t.dataset.gdNodeCheck && n) {
        n[t.dataset.gdNodeCheck] = t.checked; dirty(); renderCanvas(); renderTest();
        return;
      }
      if (t.dataset.gdTcol !== undefined && n) {
        const k = +t.dataset.gdTcol;
        if (n.columns.some((c, i) => c === t.value && i !== k)) { alert("Essa coluna já existe na tabela."); t.value = n.columns[k]; return; }
        n.columns[k] = t.value; dirty(); renderCanvas(); renderSide(); renderTest();
        return;
      }
      if (t.dataset.gdTrow !== undefined && n && t.tagName === "SELECT") {
        const row = n.rows[+t.dataset.gdTrow];
        while (row.values.length < n.columns.length) row.values.push("");
        row.values[+t.dataset.col] = t.value; dirty(); renderCanvas(); renderTest();
        return;
      }
      if (t.dataset.gdUpd !== undefined && n && t.tagName === "SELECT") {
        n.set[+t.dataset.gdUpd][t.dataset.k] = t.value; dirty(); renderCanvas(); renderSide(); renderTest();
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

    const exp = document.querySelector("[data-gd-flow-export]");
    if (exp) exp.addEventListener("click", exportFile);
    const imp = document.querySelector("[data-gd-flow-import]");
    if (imp) imp.addEventListener("click", () => $("gd-flow-import-file").click());

    const form = $("gd-flow-form");
    if (form) form.addEventListener("submit", (e) => {
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
