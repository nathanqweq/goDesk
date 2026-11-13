<?php
/**
 * Visualização somente leitura da dependência v2 (godesk.topology.view).
 * Usa o mesmo godesk-topology.js do editor, em modo readonly (data.readonly):
 * só o modo Simular, sem arrastar, sem campos editáveis, sem salvar.
 */
(new CHtmlPage())
	->setTitle($data['title'])
	->show();

function h($s) { return htmlspecialchars((string)$s, ENT_QUOTES, 'UTF-8'); }

echo '<div class="godesk-module">';
echo '<div class="gd-wrap gd-wrap-wide">';

echo '<div class="gd-header">';
echo '<div class="gd-title">'.h($data['title']).'</div>';
echo '<div class="gd-subtitle">Somente leitura — a simulação não altera nada.</div>';
echo '</div>';

if (!empty($data['error'])) {
	echo '<div class="gd-banner gd-err"><b>Erro:</b> '.h($data['error']).'</div>';
}

echo '<div class="gd-card">';
echo '<h2>🧭 Como ler</h2>';
echo '<div class="gd-muted">';
echo 'Cada localidade mostra por onde o Zabbix enxerga os equipamentos (a partir do <b>root</b>). Quando algo cai, o goDesk decide quem abre chamado:<br>';
echo '<b>1. Status da localidade</b>: com o evento de status ativo (ex: <i>Powerplant Down</i>), só o host de status abre chamado — os demais ficam segurados.<br>';
echo '<b>2. Topologia</b>: um host DOWN só abre chamado se ainda houver caminho do root até um vizinho dele por hosts UP. Se o caminho foi cortado por outro equipamento DOWN, ele fica segurado (é consequência).<br>';
echo 'Ao abrir, a página carrega o <b>estado atual do Zabbix</b>. Clique nos hosts para simular outras quedas.';
echo '</div>';
echo '</div>';

echo '<div class="gd-card" id="gd-topo">';

echo '<div class="gd-row">';
echo '<div class="gd-field gd-field-tight"><label>Localidade</label><select id="gd-topo-site"></select></div>';
echo '<div class="gd-field"><label>Resumo</label><div class="gd-topo-ro-info" id="gd-topo-ro-info"></div></div>';
echo '</div>';

echo '<div class="gd-divider"></div>';

echo '<div class="gd-topo-simbar">';
echo '<label class="gd-check"><input type="checkbox" id="gd-topo-sim-sitedown"> <span>Evento de status ativo (<span class="gd-topo-sim-event">Powerplant Down</span>)</span></label>';
echo '<button class="gd-btn" type="button" data-gd-topo="sim-live">📡 Carregar estado atual do Zabbix</button>';
echo '<button class="gd-btn" type="button" data-gd-topo="sim-clear">Limpar</button>';
echo '<span class="gd-muted gd-topo-sim-hint">Clique nos hosts para marcar/desmarcar DOWN.</span>';
echo '</div>';

echo '<div class="gd-topo-main">';
echo '<div class="gd-topo-canvas-wrap"><svg id="gd-topo-svg" class="gd-topo-svg" xmlns="http://www.w3.org/2000/svg"></svg></div>';
echo '<div class="gd-topo-side" id="gd-topo-side"></div>';
echo '</div>';
echo '<div class="gd-topo-legend" id="gd-topo-legend"></div>';

echo '</div>';

echo '<script type="application/json" id="gd-topo-data">'.json_encode([
	'sites' => $data['sites'] ?? [],
	'defaults' => $data['defaults'] ?? [],
	'readonly' => true
], JSON_UNESCAPED_UNICODE | JSON_HEX_TAG | JSON_HEX_AMP | JSON_HEX_APOS | JSON_HEX_QUOT).'</script>';

if (!empty($data['can_edit'])) {
	echo '<div class="gd-actions">';
	echo '<a class="gd-btn" href="zabbix.php?action=godesk.topology.edit">✏️ Editar arquitetura</a>';
	echo '</div>';
}

echo '</div></div>';
