(function () {
  "use strict";

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

  function removeClient(btn) {
    const card = btn.closest(".gd-client");
    if (card) card.remove();
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
            <label>Client (nome do cliente)</label>
            <input type="text" name="clients[${i}][client]" value="">
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
          <div class="gd-field gd-field-tight">
            <label>Autoclose</label>
            <div class="gd-check">
              <input type="checkbox" class="gd-autoclose" name="clients[${i}][autoclose]" value="1">
            </div>
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
      </div>
    `;

    host.insertAdjacentHTML("beforeend", html);

    // como por padrão autoclose vem desligado, trava SLA
    initSlaLocks();
  }

  // Event delegation (sem precisar onclick inline)
  document.addEventListener("click", (e) => {
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
  });

  document.addEventListener("change", (e) => {
    const chk = e.target.closest(".gd-autoclose");
    if (chk) toggleSlaFromCheckbox(chk);
  });

  document.addEventListener("DOMContentLoaded", initSlaLocks);
})();
