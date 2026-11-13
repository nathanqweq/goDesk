<?php
(new CHtmlPage())
	->setTitle($data['title'])
	->show();

function h($s) { return htmlspecialchars((string)$s, ENT_QUOTES, 'UTF-8'); }

$hosts = $data['hosts'] ?? [];
$groups = $data['groups'] ?? [];

echo '<div class="godesk-module">';
echo '<div class="gd-wrap gd-wrap-wide">';

echo '<div class="gd-header">';
echo '<div class="gd-title">'.h($data['title']).'</div>';
echo '<div class="gd-subtitle"><b>Arquivo:</b> '.h($data['path'] ?? '').'</div>';
echo '</div>';

if (!empty($data['status'])) {
	echo '<div class="gd-banner gd-ok">'.h($data['status']).'</div>';
	if (!empty($data['backup'])) {
		echo '<div class="gd-subtitle"><b>Backup:</b> '.h($data['backup']).'</div>';
	}
}
if (!empty($data['warning'])) {
	echo '<div class="gd-banner gd-warn"><b>Atenção:</b> '.h($data['warning']).'</div>';
}
if (!empty($data['error'])) {
	echo '<div class="gd-banner gd-err"><b>Erro:</b> '.h($data['error']).'</div>';
}
if (!empty($data['sync_status'])) {
	echo '<div class="gd-banner gd-ok">'.h($data['sync_status']).'</div>';
}
if (!empty($data['sync_error'])) {
	echo '<div class="gd-banner gd-err"><b>Sincronização:</b> '.h($data['sync_error']).'</div>';
}

echo '<div class="gd-card">';
echo '<h2>🧭 Como funciona</h2>';
echo '<div class="gd-muted">';
echo 'A v2 descreve a <b>arquitetura</b> de cada localidade e roda em paralelo à <a href="zabbix.php?action=godesk.dependencies.edit">v1</a>. Cada rule escolhe a versão em <a href="zabbix.php?action=godesk.config.edit">Config</a> (<b>v1</b>, <b>v2</b> ou <b>shadow</b>: a v1 decide e a v2 só registra no log o que faria — <code>[dependency-shadow]</code>).<br>';
echo '<b>1. Status da localidade</b>: com o evento de status ativo (ex: <i>Powerplant Down</i>), nenhum host da localidade abre chamado — só o host de status.<br>';
echo '<b>2. Topologia</b>: um host só abre chamado se ainda existir caminho do root (Zabbix) até um vizinho dele passando apenas por hosts UP. Hosts DOWN = com o <i>evento de DOWN</i> ativo (padrão: ping).<br>';
echo 'Use <b>Simular</b> para marcar hosts como DOWN e ver quais abririam chamado antes de salvar.';
echo '</div>';
echo '<div class="gd-actions">';
echo '<button class="gd-btn" type="button" data-gd-topo-export>⬇ Exportar YAML</button>';
echo '<button class="gd-btn" type="button" data-gd-topo-import>⬆ Importar YAML</button>';
echo '<input type="file" id="gd-topo-import-file" accept=".yaml,.yml,text/yaml" style="display:none">';
echo '</div>';
echo '</div>';

echo '<textarea id="gd-topo-export-yaml" style="display:none">'.h($data['raw'] ?? '').'</textarea>';

echo '<form id="gd-topo-import-form" method="post" action="zabbix.php?action=godesk.dependencies.v2.edit" style="display:none">';
echo '<input type="hidden" name="import" value="1">';
echo '<textarea name="import_yaml"></textarea>';
echo '</form>';

echo '<datalist id="gd-topo-hosts">';
foreach ($hosts as $hh) {
	$label = ($hh['name'] ?? '') !== ($hh['host'] ?? '') ? $hh['name'] : '';
	echo '<option value="'.h($hh['host'] ?? '').'"'.($label !== '' ? ' label="'.h($label).'"' : '').'></option>';
}
echo '</datalist>';
echo '<datalist id="gd-topo-groups">';
foreach ($groups as $g) {
	echo '<option value="'.h($g).'"></option>';
}
echo '</datalist>';

// ===== editor =====
echo '<div class="gd-card" id="gd-topo">';

echo '<div class="gd-row">';
echo '<div class="gd-field gd-field-tight"><label>Localidade</label><select id="gd-topo-site"></select></div>';
echo '<div class="gd-field gd-field-tight" style="min-width:auto;"><button class="gd-btn" type="button" data-gd-topo="site-add">+ Nova localidade</button></div>';
echo '<div class="gd-field gd-field-tight" style="min-width:auto;"><button class="gd-btn gd-btn-danger" type="button" data-gd-topo="site-remove">Remover localidade</button></div>';
echo '</div>';

echo '<div class="gd-row gd-topo-siteprops" style="margin-top:10px;">';
echo '<div class="gd-field gd-field-tight"><label>Nome</label><input type="text" data-gd-site-field="name" placeholder="UHCC"></div>';
echo '<div class="gd-field gd-field-tight"><label>Root (entrada do Zabbix)</label><input type="text" data-gd-site-field="root" placeholder="ZABBIX"></div>';
echo '<div class="gd-field"><label>Host de status</label><input type="text" data-gd-site-field="status.host" list="gd-topo-hosts" autocomplete="off" placeholder="UHCC - Core Status"></div>';
echo '<div class="gd-field gd-field-tight"><label>Evento de status <span class="gd-muted">(vazio = qualquer)</span></label><input type="text" data-gd-site-field="status.event" placeholder="Powerplant Down"></div>';
echo '<div class="gd-field gd-field-tight"><label>Evento de DOWN <span class="gd-muted">(aceita *)</span></label><input type="text" data-gd-site-field="down_event" placeholder="*Unavailable by ICMP ping*"></div>';
echo '</div>';

echo '<div class="gd-divider"></div>';

echo '<div class="gd-row gd-topo-toolbar">';
echo '<div class="gd-topo-modes">';
echo '<button class="gd-btn gd-topo-mode" type="button" data-gd-topo-mode="edit" title="Arrastar, selecionar, Delete remove">✋ Editar</button>';
echo '<button class="gd-btn gd-topo-mode" type="button" data-gd-topo-mode="link" title="Clique em dois hosts para ligar/desligar">🔗 Ligar</button>';
echo '<button class="gd-btn gd-topo-mode" type="button" data-gd-topo-mode="sim" title="Clique nos hosts para marcar DOWN">🧪 Simular</button>';
echo '</div>';
echo '<div class="gd-field"><label>Adicionar host</label><input type="text" id="gd-topo-newhost" list="gd-topo-hosts" autocomplete="off" placeholder="nome técnico do host"></div>';
echo '<div class="gd-field gd-field-tight" style="min-width:90px;"><label>Rack</label><input type="text" id="gd-topo-newrack" placeholder="1"></div>';
echo '<div class="gd-field gd-field-tight" style="min-width:auto;"><button class="gd-btn" type="button" data-gd-topo="node-add">+ Host</button></div>';
echo '<div class="gd-field"><label>Importar hosts de um grupo</label><input type="text" id="gd-topo-group" list="gd-topo-groups" autocomplete="off" placeholder="POWERPLANT/GERACAO/UHCC - ..."></div>';
echo '<div class="gd-field gd-field-tight" style="min-width:auto;"><button class="gd-btn" type="button" data-gd-topo="group-import">⬇ Importar</button></div>';
echo '<div class="gd-field gd-field-tight" style="min-width:auto;"><button class="gd-btn" type="button" data-gd-topo="layout">🪄 Organizar</button></div>';
echo '</div>';

echo '<div class="gd-topo-simbar" style="display:none;">';
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
	'defaults' => $data['defaults'] ?? []
], JSON_UNESCAPED_UNICODE | JSON_HEX_TAG | JSON_HEX_AMP | JSON_HEX_APOS | JSON_HEX_QUOT).'</script>';

echo '<form method="post" id="gd-topo-form" action="zabbix.php?action=godesk.dependencies.v2.edit">';
echo '<input type="hidden" name="save" value="1">';
echo '<textarea name="topology_json" id="gd-topo-json" style="display:none"></textarea>';
echo '<div class="gd-actions">';
echo '<button class="gd-btn gd-btn-primary" type="submit">💾 Salvar</button>';
echo '<a class="gd-btn" href="zabbix.php?action=godesk.dependencies.v2.edit">↩ Recarregar</a>';
echo '<a class="gd-btn" href="zabbix.php?action=godesk.dependencies.edit">🔗 Dependências v1</a>';
echo '<a class="gd-btn" href="zabbix.php?action=godesk.config.edit">⚙️ Config</a>';
echo '</div>';
echo '</form>';

echo '</div></div>';
