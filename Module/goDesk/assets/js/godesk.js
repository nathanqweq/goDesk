(function () {
  "use strict";

  // Filtro de busca: esconde os cards de um container cujo texto (extraído
  // por textExtractor) não contém a query digitada. Reaproveitado tanto na
  // tela de Editar (extrai de <input>/<select> ao vivo) quanto na de
  // Visualizar (extrai de data-search, já que lá é tudo texto estático).
  const filters = {};

  function makeFilter(inputId, containerId, cardSelector, textExtractor) {
    const input = document.getElementById(inputId);
    const container = document.getElementById(containerId);
    if (!input || !container) return null;

    function apply() {
      const q = input.value.trim().toLowerCase();
      container.querySelectorAll(cardSelector).forEach((card) => {
        const text = textExtractor(card).toLowerCase();
        card.style.display = (q === "" || text.includes(q)) ? "" : "none";
      });
    }

    input.addEventListener("input", apply);
    filters[inputId] = apply;
    return apply;
  }

  function clearFilter(inputId) {
    const input = document.getElementById(inputId);
    if (input) input.value = "";
    if (filters[inputId]) filters[inputId]();
  }

  function initFilters() {
    makeFilter(
      "gd-filter-named-clients",
      "gd-named-clients",
      ".gd-named-client",
      (card) => card.querySelector(".gd-named-client-name")?.value || ""
    );
    makeFilter(
      "gd-filter-rules",
      "gd-clients",
      ".gd-client",
      (card) => {
        const ruleInput = card.querySelector('input[name*="[rule_name]"]');
        const clientSelect = card.querySelector(".gd-client-select");
        return (ruleInput ? ruleInput.value : "") + " " + (clientSelect ? clientSelect.value : "");
      }
    );
    makeFilter(
      "gd-filter-deps",
      "gd-deps",
      ".gd-dep",
      (card) => Array.from(card.querySelectorAll(".gd-dep-host, .gd-dep-parent-host"))
        .map((i) => i.value)
        .join(" ")
    );
    makeFilter(
      "gd-filter-view-named-clients",
      "gd-view-named-clients",
      "[data-search]",
      (card) => card.getAttribute("data-search") || ""
    );
    makeFilter(
      "gd-filter-view-rules",
      "gd-view-rules",
      "[data-search]",
      (card) => card.getAttribute("data-search") || ""
    );
  }

  function closestContainer(el) {
    return el.closest(".gd-card, .gd-client-card");
  }

  function toggleSlaFromCheckbox(chk) {
    const container = closestContainer(chk);
    if (!container) return;

    const sla = container.querySelector(".gd-sla");
    if (!sla) return;

    if (chk.checked) {
      sla.disabled = false;
      sla.style.opacity = "1";
    } else {
      sla.disabled = true;
      sla.style.opacity = "0.55";
    }
  }

  function initSlaLocks() {
    document.querySelectorAll(".gd-autoclose").forEach(toggleSlaFromCheckbox);
  }

  function toggleSendMore(chk) {
    const container = closestContainer(chk);
    if (!container) return;

    const box = container.querySelector(".gd-sendmore-box");
    const txt = container.querySelector(".gd-sendmore-text");
    if (!box || !txt) return;

    if (chk.checked) {
      box.style.display = "";
      txt.disabled = false;
      txt.style.opacity = "1";
    } else {
      box.style.display = "none";
      txt.disabled = true;
      txt.style.opacity = "0.55";
    }
  }

  function initSendMoreToggles() {
    document.querySelectorAll(".gd-sendmore-toggle").forEach(toggleSendMore);
  }

  function toggleCustomStatus(chk) {
    const container = closestContainer(chk);
    if (!container) return;

    const box = container.querySelector(".gd-customstatus-box");
    if (!box) return;

    const inputs = box.querySelectorAll("input");
    if (chk.checked) {
      box.style.display = "";
      inputs.forEach((inp) => { inp.disabled = false; inp.style.opacity = "1"; });
    } else {
      box.style.display = "none";
      inputs.forEach((inp) => { inp.disabled = true; inp.style.opacity = "0.55"; });
    }
  }

  function initCustomStatusToggles() {
    document.querySelectorAll(".gd-customstatus-toggle").forEach(toggleCustomStatus);
  }

  function toggleSendEmail(chk) {
    const container = closestContainer(chk);
    if (!container) return;

    const box = container.querySelector(".gd-sendemail-box");
    const to = container.querySelector(".gd-email-to");
    const cc = container.querySelector(".gd-email-cc");
    if (!box || !to || !cc) return;

    if (chk.checked) {
      box.style.display = "";
      to.disabled = false;
      cc.disabled = false;
      to.style.opacity = "1";
      cc.style.opacity = "1";
    } else {
      box.style.display = "none";
      to.disabled = true;
      cc.disabled = true;
      to.style.opacity = "0.55";
      cc.style.opacity = "0.55";
    }
  }

  function initSendEmailToggles() {
    document.querySelectorAll(".gd-sendemail-toggle").forEach(toggleSendEmail);
  }

  function toggleSendTeams(chk) {
    const container = closestContainer(chk);
    if (!container) return;

    const box = container.querySelector(".gd-sendteams-box");
    if (!box) return;

    const inputs = box.querySelectorAll("input");
    if (chk.checked) {
      box.style.display = "";
      inputs.forEach((inp) => { inp.disabled = false; inp.style.opacity = "1"; });
    } else {
      box.style.display = "none";
      inputs.forEach((inp) => { inp.disabled = true; inp.style.opacity = "0.55"; });
    }
  }

  function initSendTeamsToggles() {
    document.querySelectorAll(".gd-sendteams-toggle").forEach(toggleSendTeams);
  }

  function removeClient(btn) {
    const card = btn.closest(".gd-client");
    if (card) card.remove();
    syncClientSelects();
  }

  function nextClientIndex() {
    // pega o maior índice já usado e soma +1 (seguro mesmo se apagar cards)
    let max = -1;
    document.querySelectorAll(".gd-client").forEach((el) => {
      const idx = parseInt(el.getAttribute("data-idx") || "-1", 10);
      if (!Number.isNaN(idx) && idx > max) max = idx;
    });
    return max + 1;
  }

  function removeNamedClient(btn) {
    const card = btn.closest(".gd-named-client");
    if (card) card.remove();
    syncClientSelects();
  }

  function nextNamedClientIndex() {
    let max = -1;
    document.querySelectorAll(".gd-named-client").forEach((el) => {
      const idx = parseInt(el.getAttribute("data-idx") || "-1", 10);
      if (!Number.isNaN(idx) && idx > max) max = idx;
    });
    return max + 1;
  }

  // Chama godesk.config.validate (que por sua vez passa pelo godesk-client
  // no servidor, o único lugar que tem as credenciais do TopDesk) pra
  // conferir se o operator/oper_group digitados existem de verdade.
  function testNamedClient(btn) {
    const card = btn.closest(".gd-named-client");
    if (!card) return;

    const result = card.querySelector(".gd-test-result");
    const operatorInput = card.querySelector('input[name*="[topdesk][operator]"]');
    const operGroupInput = card.querySelector('input[name*="[topdesk][oper_group]"]');
    const operator = operatorInput ? operatorInput.value.trim() : "";
    const operGroup = operGroupInput ? operGroupInput.value.trim() : "";

    if (!operator && !operGroup) {
      if (result) {
        result.className = "gd-test-result gd-muted";
        result.textContent = "Preencha operator e/ou oper_group pra testar.";
      }
      return;
    }

    if (result) {
      result.className = "gd-test-result gd-muted";
      result.textContent = "Testando...";
    }
    btn.disabled = true;

    fetch("zabbix.php?action=godesk.config.validate", {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: new URLSearchParams({ operator, oper_group: operGroup })
    })
      .then((r) => r.json())
      .then((data) => {
        if (!result) return;

        if (data.error) {
          result.className = "gd-test-result gd-error";
          result.textContent = "❌ " + data.error;
          return;
        }

        const parts = [];
        if (operator) {
          const r1 = data.operator;
          parts.push(r1 && r1.valid ? `✅ operator (${escapeHtml(operator)})` : `❌ operator (${escapeHtml(operator)}): ${escapeHtml((r1 && r1.error) || "inválido")}`);
        }
        if (operGroup) {
          const r2 = data.oper_group;
          parts.push(r2 && r2.valid ? `✅ oper_group (${escapeHtml(operGroup)})` : `❌ oper_group (${escapeHtml(operGroup)}): ${escapeHtml((r2 && r2.error) || "inválido")}`);
        }

        const anyInvalid = (operator && !(data.operator && data.operator.valid)) || (operGroup && !(data.oper_group && data.oper_group.valid));
        result.className = "gd-test-result " + (anyInvalid ? "gd-error" : "gd-ok");
        result.innerHTML = parts.join("<br>");
      })
      .catch((err) => {
        if (!result) return;
        result.className = "gd-test-result gd-error";
        result.textContent = "❌ falha ao testar: " + err;
      })
      .finally(() => {
        btn.disabled = false;
      });
  }

  function escapeHtml(s) {
    return String(s)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  // Mantém as opções de "Cliente" de toda rule em sincronia com os nomes
  // atualmente digitados nos cards de Clientes (adicionar/remover/renomear
  // um cliente reflete em todos os <select> na hora).
  function currentClientNames() {
    const names = new Set();
    document.querySelectorAll(".gd-named-client-name").forEach((inp) => {
      const v = inp.value.trim();
      if (v) names.add(v);
    });
    return Array.from(names).sort();
  }

  function syncClientSelects() {
    const names = currentClientNames();
    document.querySelectorAll(".gd-client-select").forEach((sel) => {
      const current = sel.value;
      const orphanCurrent = current && !names.includes(current);

      let html = '<option value="">— nenhum / customizado —</option>';
      names.forEach((n) => {
        const selected = n === current ? " selected" : "";
        html += `<option value="${escapeHtml(n)}"${selected}>${escapeHtml(n)}</option>`;
      });
      if (orphanCurrent) {
        html += `<option value="${escapeHtml(current)}" selected>${escapeHtml(current)} (não cadastrado)</option>`;
      }

      sel.innerHTML = html;
    });
  }

  function addNamedClient() {
    const host = document.getElementById("gd-named-clients");
    if (!host) return;

    const i = nextNamedClientIndex();

    const html = `
      <div class="gd-client-card gd-named-client" data-idx="${i}">
        <div class="gd-client-head">
          <div class="gd-client-name">🏢 Cliente</div>
          <div>
            <button class="gd-btn" type="button" data-gd-test-client>🔍 Testar</button>
            <button class="gd-btn gd-btn-danger" type="button" data-gd-remove-named-client>Remover</button>
          </div>
        </div>

        <div class="gd-row">
          <div class="gd-field">
            <label>Nome do cliente (chave do YAML)</label>
            <input type="text" class="gd-named-client-name" name="named_clients[${i}][name]" value="">
          </div>
        </div>

        <div class="gd-test-result gd-muted" style="margin-bottom:8px"></div>

        <div class="gd-divider"></div>
        <div class="gd-small-title">🎫 TopDesk (compartilhado por todas as rules deste cliente)</div>

        <div class="gd-row">
          <div class="gd-field"><label>contract</label><input type="text" name="named_clients[${i}][topdesk][contract]" value=""></div>
          <div class="gd-field"><label>operator</label><input type="text" name="named_clients[${i}][topdesk][operator]" value=""></div>
          <div class="gd-field"><label>oper_group</label><input type="text" name="named_clients[${i}][topdesk][oper_group]" value=""></div>
        </div>

        <div class="gd-row">
          <div class="gd-field"><label>main_caller</label><input type="text" name="named_clients[${i}][topdesk][main_caller]" value=""></div>
          <div class="gd-field"><label>secundary_caller</label><input type="text" name="named_clients[${i}][topdesk][secundary_caller]" value=""></div>
          <div class="gd-field"><label>sla</label><input type="text" class="gd-sla" name="named_clients[${i}][topdesk][sla]" value=""></div>
        </div>

        <div class="gd-row">
          <div class="gd-field"><label>category</label><input type="text" name="named_clients[${i}][topdesk][category]" value=""></div>
          <div class="gd-field"><label>sub_category</label><input type="text" name="named_clients[${i}][topdesk][sub_category]" value=""></div>
          <div class="gd-field"><label>call_type</label><input type="text" name="named_clients[${i}][topdesk][call_type]" value=""></div>
        </div>

        <div class="gd-row">
          <div class="gd-field gd-field-tight">
            <label>Sendmore info</label>
            <div class="gd-check">
              <input type="checkbox" class="gd-sendmore-toggle" name="named_clients[${i}][topdesk][send_more_info]" value="1">
              <span class="gd-muted">comentar após criar o chamado</span>
            </div>
          </div>
        </div>

        <div class="gd-row gd-sendmore-box" style="display:none">
          <div class="gd-field">
            <label>Texto sendmore (padrão do cliente)</label>
            <textarea class="gd-sendmore-text" name="named_clients[${i}][topdesk][more_info_text]" rows="4" disabled></textarea>
          </div>
        </div>

        <div class="gd-row">
          <div class="gd-field gd-field-tight">
            <label>Adicional cresol</label>
            <div class="gd-check">
              <input type="checkbox" name="named_clients[${i}][topdesk][adicional_cresol]" value="1">
              <span class="gd-muted">enviar optionalFields1 no create</span>
            </div>
          </div>
        </div>

        <div class="gd-row">
          <div class="gd-field gd-field-tight">
            <label>Enviar email</label>
            <div class="gd-check">
              <input type="checkbox" class="gd-sendemail-toggle" name="named_clients[${i}][topdesk][send_email]" value="1">
              <span class="gd-muted">enviar email apos criar o chamado</span>
            </div>
          </div>
        </div>

        <div class="gd-row gd-sendemail-box" style="display:none">
          <div class="gd-field">
            <label>Email para</label>
            <input type="text" class="gd-email-to" name="named_clients[${i}][topdesk][email_to]" value="" disabled>
          </div>
          <div class="gd-field">
            <label>Email copia</label>
            <input type="text" class="gd-email-cc" name="named_clients[${i}][topdesk][email_cc]" value="" disabled>
          </div>
          <div class="gd-field gd-field-tight">
            <label>Um por dia</label>
            <div class="gd-check">
              <input type="checkbox" name="named_clients[${i}][topdesk][once_per_day]" value="1">
              <span class="gd-muted">no máx. 1 e-mail/dia por alerta+host</span>
            </div>
          </div>
        </div>
      </div>
    `;

    host.insertAdjacentHTML("beforeend", html);

    initSlaLocks();
    initSendMoreToggles();
    initSendEmailToggles();
    initSendTeamsToggles();
    syncClientSelects();
    clearFilter("gd-filter-named-clients");
  }

  function addClient() {
    const host = document.getElementById("gd-clients");
    if (!host) return;

    const i = nextClientIndex();

    const html = `
      <div class="gd-client-card gd-client" data-idx="${i}">
        <div class="gd-client-head">
          <div class="gd-client-name">🧩 Rule</div>
          <button class="gd-btn gd-btn-danger" type="button" data-gd-remove>Remover</button>
        </div>

        <div class="gd-row">
          <div class="gd-field">
            <label>Rule name (chave do YAML)</label>
            <input type="text" name="clients[${i}][rule_name]" value="">
          </div>
          <div class="gd-field">
            <label>Cliente</label>
            <select class="gd-client-select" name="clients[${i}][client]">
              <option value="">— nenhum / customizado —</option>
            </select>
          </div>
          <div class="gd-field">
            <label>Urgency</label>
            <input type="text" name="clients[${i}][urgency]" value="">
          </div>
        </div>

        <div class="gd-row">
          <div class="gd-field">
            <label>Impact</label>
            <input type="text" name="clients[${i}][impact]" value="">
          </div>
          <div class="gd-field">
            <label>Priority</label>
            <input type="text" name="clients[${i}][priority]" value="">
          </div>
          <div class="gd-field gd-field-tight">
            <label>Autoclose</label>
            <div class="gd-check">
              <input type="checkbox" class="gd-autoclose" name="clients[${i}][autoclose]" value="1">
            </div>
          </div>
        </div>

        <div class="gd-row">
          <div class="gd-field gd-field-tight">
            <label>Custom status</label>
            <div class="gd-check">
              <input type="checkbox" class="gd-customstatus-toggle" name="clients[${i}][custom_status]" value="1">
              <span class="gd-muted">sobrescrever o processingStatus padrão de abertura/atualização</span>
            </div>
          </div>
          <div class="gd-field gd-field-tight">
            <label>Dependência</label>
            <div class="gd-check">
              <input type="checkbox" name="clients[${i}][dependency]" value="1">
              <span class="gd-muted">não abrir chamado se um host pai estiver em alerta (<a href="zabbix.php?action=godesk.dependencies.edit">Dependências</a>)</span>
            </div>
          </div>
        </div>

        <div class="gd-row gd-customstatus-box" style="display:none">
          <div class="gd-field">
            <label>Status abertura (ID no TopDesk)</label>
            <input type="text" name="clients[${i}][status_open]" value="" disabled>
          </div>
          <div class="gd-field">
            <label>Status atualização (ID no TopDesk)</label>
            <input type="text" name="clients[${i}][status_update]" value="" disabled>
          </div>
        </div>

        <div class="gd-divider"></div>
        <div class="gd-small-title">🎫 TopDesk</div>

        <div class="gd-row">
          <div class="gd-field"><label>contract</label><input type="text" name="clients[${i}][topdesk][contract]" value=""></div>
          <div class="gd-field"><label>operator</label><input type="text" name="clients[${i}][topdesk][operator]" value=""></div>
          <div class="gd-field"><label>oper_group</label><input type="text" name="clients[${i}][topdesk][oper_group]" value=""></div>
        </div>

        <div class="gd-row">
          <div class="gd-field"><label>main_caller</label><input type="text" name="clients[${i}][topdesk][main_caller]" value=""></div>
          <div class="gd-field"><label>secundary_caller</label><input type="text" name="clients[${i}][topdesk][secundary_caller]" value=""></div>
          <div class="gd-field"><label>sla</label><input type="text" class="gd-sla" name="clients[${i}][topdesk][sla]" value=""></div>
        </div>

        <div class="gd-row">
          <div class="gd-field"><label>category</label><input type="text" name="clients[${i}][topdesk][category]" value=""></div>
          <div class="gd-field"><label>sub_category</label><input type="text" name="clients[${i}][topdesk][sub_category]" value=""></div>
          <div class="gd-field"><label>call_type</label><input type="text" name="clients[${i}][topdesk][call_type]" value=""></div>
        </div>

        <div class="gd-row">
          <div class="gd-field gd-field-tight">
            <label>Sendmore info</label>
            <div class="gd-check">
              <input type="checkbox" class="gd-sendmore-toggle" name="clients[${i}][topdesk][send_more_info]" value="1">
              <span class="gd-muted">comentar após criar o chamado (com cliente escolhido: envia se o cliente OU esta rule estiver marcado)</span>
            </div>
          </div>
        </div>

        <div class="gd-row gd-sendmore-box" style="display:none">
          <div class="gd-field">
            <label>Texto sendmore (puro)</label>
            <textarea class="gd-sendmore-text" name="clients[${i}][topdesk][more_info_text]" rows="4" disabled></textarea>
          </div>
        </div>

        <div class="gd-row">
          <div class="gd-field gd-field-tight">
            <label>Adicional cresol</label>
            <div class="gd-check">
              <input type="checkbox" name="clients[${i}][topdesk][adicional_cresol]" value="1">
              <span class="gd-muted">enviar optionalFields1 no create</span>
            </div>
          </div>
        </div>

        <div class="gd-row">
          <div class="gd-field gd-field-tight">
            <label>Enviar email</label>
            <div class="gd-check">
              <input type="checkbox" class="gd-sendemail-toggle" name="clients[${i}][topdesk][send_email]" value="1">
              <span class="gd-muted">enviar email apos criar o chamado</span>
            </div>
          </div>
        </div>

        <div class="gd-row gd-sendemail-box" style="display:none">
          <div class="gd-field">
            <label>Email para</label>
            <input type="text" class="gd-email-to" name="clients[${i}][topdesk][email_to]" value="" disabled>
          </div>
          <div class="gd-field">
            <label>Email copia</label>
            <input type="text" class="gd-email-cc" name="clients[${i}][topdesk][email_cc]" value="" disabled>
          </div>
          <div class="gd-field gd-field-tight">
            <label>Um por dia</label>
            <div class="gd-check">
              <input type="checkbox" name="clients[${i}][topdesk][once_per_day]" value="1">
              <span class="gd-muted">no máx. 1 e-mail/dia por alerta+host</span>
            </div>
          </div>
        </div>

        <div class="gd-row">
          <div class="gd-field gd-field-tight">
            <label>Enviar Teams</label>
            <div class="gd-check">
              <input type="checkbox" class="gd-sendteams-toggle" name="clients[${i}][topdesk][send_teams]" value="1">
              <span class="gd-muted">postar no MS Teams apos criar o chamado (vale com ou sem cliente escolhido)</span>
            </div>
          </div>
        </div>

        <div class="gd-row gd-sendteams-box" style="display:none">
          <div class="gd-field">
            <label>Webhook do Teams (Workflow)</label>
            <input type="text" class="gd-teams-webhook" name="clients[${i}][topdesk][teams_webhook]" value="" placeholder="https://...logic.azure.com/workflows/..." disabled>
          </div>
          <div class="gd-field">
            <label>URL do Zabbix (frontend)</label>
            <input type="text" class="gd-teams-zabbix-url" name="clients[${i}][topdesk][teams_zabbix_url]" value="" placeholder="https://zabbix.empresa.com (vazio = GODESK_ZABBIX_URL)" disabled>
          </div>
        </div>
      </div>
    `;

    host.insertAdjacentHTML("beforeend", html);

    // como por padrão autoclose vem desligado, trava SLA
    initSlaLocks();
    initSendMoreToggles();
    initSendEmailToggles();
    initSendTeamsToggles();
    initCustomStatusToggles();
    syncClientSelects();
    clearFilter("gd-filter-rules");
  }

  // ===== Dependências (godesk.dependencies.edit) =====

  function escapeAttr(s) {
    return String(s)
      .replace(/&/g, "&amp;")
      .replace(/"/g, "&quot;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;");
  }

  function parentRowHtml(i, j) {
    const ev = `gd-ev-${i}-${j}`;
    return `
      <div class="gd-row gd-dep-parent">
        <div class="gd-field">
          <label>Host pai</label>
          <input type="text" class="gd-dep-parent-host" list="gd-hosts-list" autocomplete="off" name="deps[${i}][parents][${j}][host]" value="">
        </div>
        <div class="gd-field">
          <label>Evento a observar <span class="gd-muted">(vazio = qualquer)</span></label>
          <input type="text" class="gd-dep-parent-event" list="${ev}" autocomplete="off" name="deps[${i}][parents][${j}][event]" value="" placeholder="ex: Unavailable by ICMP ping, *Interface*down*">
          <datalist id="${ev}"></datalist>
        </div>
        <div class="gd-field gd-field-tight" style="min-width:auto;">
          <button class="gd-btn gd-btn-danger" type="button" data-gd-dep-parent-remove>✕</button>
        </div>
      </div>
    `;
  }

  function addDependency() {
    const host = document.getElementById("gd-deps");
    if (!host) return;

    const i = parseInt(host.dataset.nextIdx || "0", 10);
    host.dataset.nextIdx = String(i + 1);

    host.insertAdjacentHTML("beforeend", `
      <div class="gd-client-card gd-dep" data-idx="${i}" data-next-parent="1">
        <div class="gd-client-head">
          <div class="gd-client-name">🖥️ Host dependente</div>
          <button class="gd-btn gd-btn-danger" type="button" data-gd-dep-remove>Remover</button>
        </div>
        <div class="gd-row">
          <div class="gd-field">
            <label>Host (nome técnico)</label>
            <input type="text" class="gd-dep-host" list="gd-hosts-list" autocomplete="off" name="deps[${i}][host]" value="">
          </div>
          <div class="gd-field gd-field-tight">
            <label>Modo</label>
            <select name="deps[${i}][mode]">
              <option value="any" selected>Qualquer pai em alerta</option>
              <option value="all">Todos os pais em alerta</option>
            </select>
          </div>
        </div>
        <div class="gd-small-title" style="margin-top:12px;">Depende de</div>
        <div class="gd-dep-parents">${parentRowHtml(i, 0)}</div>
        <div class="gd-actions"><button class="gd-btn" type="button" data-gd-dep-parent-add>+ Adicionar pai</button></div>
      </div>
    `);

    clearFilter("gd-filter-deps");
    host.lastElementChild.querySelector(".gd-dep-host")?.focus();
  }

  function addParent(btn) {
    const card = btn.closest(".gd-dep");
    if (!card) return;

    const i = card.dataset.idx;
    const j = parseInt(card.dataset.nextParent || "0", 10);
    card.dataset.nextParent = String(j + 1);

    card.querySelector(".gd-dep-parents").insertAdjacentHTML("beforeend", parentRowHtml(i, j));
  }

  // Sugere os nomes de trigger do host pai no campo "Evento a observar"
  // (busca na primeira vez que o campo recebe foco, e de novo se o host
  // pai mudar).
  function loadParentEvents(eventInput) {
    const row = eventInput.closest(".gd-dep-parent");
    const hostInput = row?.querySelector(".gd-dep-parent-host");
    const list = row?.querySelector("datalist");
    if (!hostInput || !list) return;

    const hostName = hostInput.value.trim();
    if (hostName === "" || list.dataset.host === hostName) return;
    list.dataset.host = hostName;

    fetch("zabbix.php?action=godesk.dependencies.triggers", {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: new URLSearchParams({ host: hostName })
    })
      .then((r) => r.json())
      .then((data) => {
        list.innerHTML = (data.triggers || [])
          .map((t) => `<option value="${escapeAttr(t)}"></option>`)
          .join("");
      })
      .catch(() => { list.dataset.host = ""; });
  }

  // Exportar/Importar (Dependências e Rules): o Exportar baixa o YAML salvo
  // que a view deixa num textarea oculto; o Importar lê o arquivo escolhido
  // e manda o texto num form oculto (o servidor valida e faz backup).
  function exportYaml(srcId, filename, emptyMsg) {
    const src = document.getElementById(srcId);
    const text = src ? src.value : "";
    if (text.trim() === "") {
      alert(emptyMsg);
      return;
    }

    const blob = new Blob([text], { type: "text/yaml" });
    const a = document.createElement("a");
    a.href = URL.createObjectURL(blob);
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(a.href);
  }

  function importYaml(fileInput, formId, confirmMsg) {
    const file = fileInput.files && fileInput.files[0];
    if (!file) return;

    const reader = new FileReader();
    reader.onload = () => {
      fileInput.value = "";
      if (!confirm(confirmMsg(file.name))) {
        return;
      }
      const form = document.getElementById(formId);
      form.querySelector('textarea[name="import_yaml"]').value = String(reader.result || "");
      form.submit();
    };
    reader.readAsText(file);
  }

  // Event delegation (sem precisar onclick inline)
  document.addEventListener("click", (e) => {
    const depAdd = e.target.closest("[data-gd-dep-add]");
    if (depAdd) {
      e.preventDefault();
      addDependency();
      return;
    }

    const depRemove = e.target.closest("[data-gd-dep-remove]");
    if (depRemove) {
      e.preventDefault();
      if (confirm("Remover este host dependente?")) depRemove.closest(".gd-dep")?.remove();
      return;
    }

    const parentAdd = e.target.closest("[data-gd-dep-parent-add]");
    if (parentAdd) {
      e.preventDefault();
      addParent(parentAdd);
      return;
    }

    const parentRemove = e.target.closest("[data-gd-dep-parent-remove]");
    if (parentRemove) {
      e.preventDefault();
      parentRemove.closest(".gd-dep-parent")?.remove();
      return;
    }

    if (e.target.closest("[data-gd-deps-export]")) {
      e.preventDefault();
      exportYaml("gd-deps-export-yaml", "godesk-dependencies.yaml", "Nenhuma dependência salva para exportar.");
      return;
    }

    if (e.target.closest("[data-gd-deps-import]")) {
      e.preventDefault();
      document.getElementById("gd-deps-import-file")?.click();
      return;
    }

    if (e.target.closest("[data-gd-rules-export]")) {
      e.preventDefault();
      exportYaml("gd-rules-export-yaml", "godesk-rules.yaml", "Nenhuma rule salva para exportar.");
      return;
    }

    if (e.target.closest("[data-gd-rules-import]")) {
      e.preventDefault();
      document.getElementById("gd-rules-import-file")?.click();
      return;
    }

    const addBtn = e.target.closest("[data-gd-add]");
    if (addBtn) {
      e.preventDefault();
      addClient();
      return;
    }

    const rmBtn = e.target.closest("[data-gd-remove]");
    if (rmBtn) {
      e.preventDefault();
      removeClient(rmBtn);
      return;
    }

    const addNamedBtn = e.target.closest("[data-gd-add-named-client]");
    if (addNamedBtn) {
      e.preventDefault();
      addNamedClient();
      return;
    }

    const rmNamedBtn = e.target.closest("[data-gd-remove-named-client]");
    if (rmNamedBtn) {
      e.preventDefault();
      removeNamedClient(rmNamedBtn);
      return;
    }

    const testBtn = e.target.closest("[data-gd-test-client]");
    if (testBtn) {
      e.preventDefault();
      testNamedClient(testBtn);
      return;
    }
  });

  document.addEventListener("input", (e) => {
    if (e.target.closest(".gd-named-client-name")) {
      syncClientSelects();
    }
  });

  document.addEventListener("focusin", (e) => {
    const ev = e.target.closest(".gd-dep-parent-event");
    if (ev) loadParentEvents(ev);
  });

  document.addEventListener("change", (e) => {
    if (e.target.id === "gd-deps-import-file") {
      importYaml(e.target, "gd-deps-import-form", (name) =>
        `Importar "${name}"? Isso SUBSTITUI todas as dependências atuais (um backup do arquivo atual é mantido no servidor).`);
      return;
    }

    if (e.target.id === "gd-rules-import-file") {
      importYaml(e.target, "gd-rules-import-form", (name) =>
        `Importar "${name}"? Isso SUBSTITUI todas as rules atuais (e os clientes, se o arquivo trouxer). Edições não salvas nesta tela serão perdidas. O Default não é alterado e um backup do arquivo atual é mantido no servidor.`);
      return;
    }

    const chk = e.target.closest(".gd-autoclose");
    if (chk) toggleSlaFromCheckbox(chk);

    const more = e.target.closest(".gd-sendmore-toggle");
    if (more) toggleSendMore(more);

    const sendEmail = e.target.closest(".gd-sendemail-toggle");
    if (sendEmail) toggleSendEmail(sendEmail);

    const sendTeams = e.target.closest(".gd-sendteams-toggle");
    if (sendTeams) toggleSendTeams(sendTeams);

    const customStatus = e.target.closest(".gd-customstatus-toggle");
    if (customStatus) toggleCustomStatus(customStatus);
  });

  document.addEventListener("DOMContentLoaded", () => {
    initSlaLocks();
    initSendMoreToggles();
    initSendEmailToggles();
    initSendTeamsToggles();
    initCustomStatusToggles();
    syncClientSelects();
    initFilters();
  });
})();
