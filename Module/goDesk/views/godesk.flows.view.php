<?php
/**
 * View: godesk.flows.view — fluxos em modo somente leitura (atendimento).
 * Usa o mesmo godesk-flows.js do editor com data.readonly: dá pra navegar,
 * clicar nos blocos e testar com problemas reais, sem editar nem salvar.
 */
(new CHtmlPage())
	->setTitle($data['title'])
	->show();

function h($s) { return htmlspecialchars((string)$s, ENT_QUOTES, 'UTF-8'); }

echo '<div class="godesk-module">';
echo '<div class="gd-wrap gd-wrap-wide">';

echo '<div class="gd-header">';
echo '<div class="gd-title">'.h($data['title']).'</div>';
echo '<div class="gd-subtitle">Somente leitura — o teste não altera nada.</div>';
echo '</div>';

if (!empty($data['error'])) {
	echo '<div class="gd-banner gd-err"><b>Erro:</b> '.h($data['error']).'</div>';
}

echo '<div class="gd-card">';
echo '<h2>🔀 Como ler</h2>';
echo '<div class="gd-muted">';
echo 'Cada fluxo decide, na hora do alerta, como o chamado é aberto: <b>Condição</b> e <b>Switch</b> escolhem o caminho pelos dados do evento (severidade, tags, horário...), <b>Tabela</b> e <b>Definir campos</b> definem urgência, grupo e a <b>instrução</b> pro atendimento, <b>Atualizar chamado</b> escala um chamado já aberto quando o alerta volta, e <b>Fim</b> decide se abre ou não.<br>';
echo 'Clique num bloco para ver os detalhes. Use <b>🧪 Testar</b> com um problema real para ver o caminho que o alerta faria.';
echo '</div>';
echo '</div>';

echo '<div class="gd-card" id="gd-flow-cals">';
echo '<h2>📅 Calendários</h2>';
echo '<div id="gd-flow-cal-list"></div>';
echo '</div>';

echo '<div class="gd-card" id="gd-flow">';
echo '<div class="gd-row">';
echo '<div class="gd-field gd-field-tight"><label>Fluxo</label><select id="gd-flow-select"></select></div>';
echo '<div class="gd-field"><label>Descrição</label><div class="gd-flow-rules" id="gd-flow-desc"></div></div>';
echo '<div class="gd-field gd-field-tight" style="min-width:auto;"><label>&nbsp;</label><button class="gd-btn" type="button" data-gd-flow="test-toggle">🧪 Testar</button></div>';
echo '</div>';

echo '<div class="gd-flow-main">';
echo '<div class="gd-flow-canvas-wrap"><svg id="gd-flow-svg" class="gd-flow-svg" xmlns="http://www.w3.org/2000/svg"></svg></div>';
echo '<div class="gd-flow-side" id="gd-flow-side"></div>';
echo '</div>';

$test_visible = true;
require __DIR__.'/inc.flows.test.php';

echo '</div>';

echo '<datalist id="gd-flow-fields"></datalist>';

echo '<script type="application/json" id="gd-flow-data">'.json_encode([
	'data' => $data['data'] ?? ['calendars' => [], 'flows' => []],
	'readonly' => true
], JSON_UNESCAPED_UNICODE | JSON_HEX_TAG | JSON_HEX_AMP | JSON_HEX_APOS | JSON_HEX_QUOT | JSON_INVALID_UTF8_SUBSTITUTE).'</script>';

if (!empty($data['can_edit'])) {
	echo '<div class="gd-actions">';
	echo '<a class="gd-btn" href="zabbix.php?action=godesk.flows.edit">✏️ Editar fluxos</a>';
	echo '<a class="gd-btn" href="zabbix.php?action=godesk.flows.history">🕘 Histórico</a>';
	echo '</div>';
}

echo '</div></div>';
