<?php
/**
 * View: godesk.flows.edit — editor visual dos fluxos de decisão das rules.
 * O editor em si é o assets/js/godesk-flows.js (só roda se #gd-flow existir).
 */
(new CHtmlPage())
	->setTitle($data['title'])
	->show();

function h($s) { return htmlspecialchars((string)$s, ENT_QUOTES, 'UTF-8'); }

echo '<div class="godesk-module">';
echo '<div class="gd-wrap gd-wrap-wide">';

echo '<div class="gd-header">';
echo '<div class="gd-title">'.h($data['title']).'</div>';
echo '<div class="gd-subtitle"><b>Arquivo:</b> '.h($data['path'] ?? '').'</div>';
echo '</div>';

foreach (['status' => 'gd-ok', 'sync_status' => 'gd-ok'] as $k => $cls) {
	if (!empty($data[$k])) {
		echo '<div class="gd-banner '.$cls.'">'.h($data[$k]).'</div>';
	}
}
if (!empty($data['backup'])) {
	echo '<div class="gd-subtitle"><b>Backup:</b> '.h($data['backup']).'</div>';
}
if (!empty($data['error'])) {
	echo '<div class="gd-banner gd-err" style="white-space:pre-wrap"><b>Erro:</b> '.h($data['error']).'</div>';
}
if (!empty($data['sync_error'])) {
	echo '<div class="gd-banner gd-err"><b>Sincronização:</b> '.h($data['sync_error']).'</div>';
}

echo '<div class="gd-card">';
echo '<h2>🔀 Como funciona</h2>';
echo '<div class="gd-muted">';
echo 'Em vez de uma rule para cada variação, a rule aponta para um <b>fluxo</b> (campo <i>Fluxo</i> em <a href="zabbix.php?action=godesk.config.edit">Config (editar)</a>) e o fluxo decide na hora do alerta.<br>';
echo '<b>Condição</b> (sim/não) e <b>Switch</b> (um caminho por valor) olham os dados do evento: severidade, grupos e tags do host, tags do evento, horário (calendários), se o chamado já existe...<br>';
echo '<b>Definir campos</b> sobrescreve o que a rule definiria (urgência, grupo operador, Teams...) e a <b>instrução</b> pro atendimento — vai no chamado (invisível pro solicitante) e no Teams. Textos aceitam variáveis: <code>{host}</code>, <code>{host.name}</code>, <code>{trigger}</code>, <code>{severity}</code>, <code>{host.tag.Site}</code>...<br>';
echo '<b>Fim</b>: abrir chamado (padrão — saída sem ligação também abre) ou <b>não abrir</b> (só comenta no evento).<br>';
echo 'Use <b>🧪 Testar</b> com um evento real antes de ativar. Na rule, o modo <b>shadow</b> roda o fluxo só no log (<code>[flow-shadow]</code>) sem mudar nada.';
echo '</div>';
echo '<div class="gd-actions">';
echo '<button class="gd-btn" type="button" data-gd-flow-export>⬇ Exportar</button>';
echo '<button class="gd-btn" type="button" data-gd-flow-import>⬆ Importar</button>';
echo '<input type="file" id="gd-flow-import-file" accept=".yaml,.yml,.json,text/yaml,application/json" style="display:none">';
echo '</div>';
echo '</div>';

echo '<textarea id="gd-flow-export-text" style="display:none">'.h($data['raw'] ?? '').'</textarea>';
echo '<form id="gd-flow-import-form" method="post" action="zabbix.php?action=godesk.flows.edit" style="display:none">';
echo '<input type="hidden" name="import" value="1"><textarea name="import_yaml"></textarea>';
echo '</form>';

// ===== calendários =====
echo '<div class="gd-card" id="gd-flow-cals">';
echo '<h2>📅 Calendários <span class="gd-muted">(usados em condições como <code>calendar.&lt;nome&gt;</code>)</span></h2>';
echo '<div id="gd-flow-cal-list"></div>';
echo '<div class="gd-actions"><button class="gd-btn" type="button" data-gd-flow="cal-add">+ Calendário</button></div>';
echo '</div>';

// ===== editor =====
echo '<div class="gd-card" id="gd-flow">';

echo '<div class="gd-row">';
echo '<div class="gd-field gd-field-tight"><label>Fluxo</label><select id="gd-flow-select"></select></div>';
echo '<div class="gd-field gd-field-tight" style="min-width:auto;"><button class="gd-btn" type="button" data-gd-flow="flow-add">+ Novo fluxo</button></div>';
echo '<div class="gd-field gd-field-tight" style="min-width:auto;"><button class="gd-btn" type="button" data-gd-flow="flow-dup">⧉ Duplicar</button></div>';
echo '<div class="gd-field gd-field-tight" style="min-width:auto;"><button class="gd-btn gd-btn-danger" type="button" data-gd-flow="flow-remove">Remover fluxo</button></div>';
echo '<div class="gd-field"><label>Usado pelas rules</label><div class="gd-flow-rules" id="gd-flow-rules"></div></div>';
echo '</div>';

echo '<div class="gd-divider"></div>';

echo '<div class="gd-row gd-flow-toolbar">';
echo '<span class="gd-muted">Adicionar:</span>';
echo '<button class="gd-btn gd-flow-add-if" type="button" data-gd-flow-add="if">◇ Condição</button>';
echo '<button class="gd-btn gd-flow-add-switch" type="button" data-gd-flow-add="switch">⑂ Switch</button>';
echo '<button class="gd-btn gd-flow-add-set" type="button" data-gd-flow-add="set">✎ Definir campos</button>';
echo '<button class="gd-btn gd-flow-add-end" type="button" data-gd-flow-add="end">■ Fim</button>';
echo '<span class="gd-flow-sep"></span>';
echo '<button class="gd-btn" type="button" data-gd-flow="layout">🪄 Organizar</button>';
echo '<button class="gd-btn" type="button" data-gd-flow="test-toggle">🧪 Testar</button>';
echo '</div>';

echo '<div class="gd-flow-main">';
echo '<div class="gd-flow-canvas-wrap"><svg id="gd-flow-svg" class="gd-flow-svg" xmlns="http://www.w3.org/2000/svg"></svg></div>';
echo '<div class="gd-flow-side" id="gd-flow-side"></div>';
echo '</div>';

// ===== teste =====
echo '<div class="gd-flow-test" id="gd-flow-test" style="display:none">';
echo '<div class="gd-small-title">🧪 Testar o fluxo <span class="gd-muted">(não altera nada — mesma decisão do goDesk na hora do alerta)</span></div>';
echo '<div class="gd-row">';
echo '<div class="gd-field"><label>Carregar de um problema recente</label><select id="gd-flow-test-recent"><option value="">— clique para carregar a lista —</option></select></div>';
echo '<div class="gd-field gd-field-tight"><label>ou Event ID</label><input type="text" id="gd-flow-test-eventid" placeholder="123456"></div>';
echo '<div class="gd-field gd-field-tight" style="min-width:auto;"><button class="gd-btn" type="button" data-gd-flow="test-load">📡 Carregar</button></div>';
echo '</div>';
echo '<div class="gd-row">';
echo '<div class="gd-field"><label>host</label><input type="text" data-gd-test="host"></div>';
echo '<div class="gd-field"><label>host.name</label><input type="text" data-gd-test="host.name"></div>';
echo '<div class="gd-field"><label>trigger</label><input type="text" data-gd-test="trigger"></div>';
echo '<div class="gd-field gd-field-tight"><label>severity</label><select data-gd-test="severity">';
foreach (['Not classified', 'Information', 'Warning', 'Average', 'High', 'Disaster'] as $sv) {
	echo '<option value="'.$sv.'"'.($sv === 'High' ? ' selected' : '').'>'.$sv.'</option>';
}
echo '</select></div>';
echo '</div>';
echo '<div class="gd-row">';
echo '<div class="gd-field gd-field-tight"><label>Tipo</label><select data-gd-test="event.kind"><option value="problem">problema</option><option value="recovery">normalização</option></select></div>';
echo '<div class="gd-field gd-field-tight"><label>Chamado já existe?</label><select data-gd-test="ticket.exists"><option value="false">não</option><option value="true">sim</option></select></div>';
echo '<div class="gd-field gd-field-tight"><label>Data/hora do alerta</label><input type="datetime-local" data-gd-test="now"></div>';
echo '<div class="gd-field"><label>rule</label><input type="text" data-gd-test="rule"></div>';
echo '</div>';
echo '<div class="gd-row">';
echo '<div class="gd-field"><label>Grupos do host <span class="gd-muted">(um por linha)</span></label><textarea rows="3" data-gd-test="host.group"></textarea></div>';
echo '<div class="gd-field"><label>Tags do host <span class="gd-muted">(Tag=Valor, uma por linha)</span></label><textarea rows="3" data-gd-test="host_tags"></textarea></div>';
echo '<div class="gd-field"><label>Tags do evento <span class="gd-muted">(Tag=Valor)</span></label><textarea rows="3" data-gd-test="event_tags"></textarea></div>';
echo '<div class="gd-field"><label>Outros campos <span class="gd-muted">(campo=valor)</span></label><textarea rows="3" data-gd-test="extra" placeholder="cliente=UHCC&#10;value=0"></textarea></div>';
echo '</div>';
echo '<div class="gd-flow-test-result" id="gd-flow-test-result"></div>';
echo '</div>';

echo '</div>';

echo '<datalist id="gd-flow-fields"></datalist>';

echo '<script type="application/json" id="gd-flow-data">'.json_encode([
	'data' => $data['data'] ?? ['calendars' => [], 'flows' => []],
	'rules_by_flow' => $data['rules_by_flow'] ?? new \stdClass()
], JSON_UNESCAPED_UNICODE | JSON_HEX_TAG | JSON_HEX_AMP | JSON_HEX_APOS | JSON_HEX_QUOT | JSON_INVALID_UTF8_SUBSTITUTE).'</script>';

echo '<form method="post" id="gd-flow-form" action="zabbix.php?action=godesk.flows.edit">';
echo '<input type="hidden" name="save" value="1">';
echo '<textarea name="flows_json" id="gd-flow-json" style="display:none"></textarea>';
echo '<div class="gd-actions">';
echo '<button class="gd-btn gd-btn-primary" type="submit">💾 Salvar</button>';
echo '<a class="gd-btn" href="zabbix.php?action=godesk.flows.edit">↩ Recarregar</a>';
echo '<a class="gd-btn" href="zabbix.php?action=godesk.config.edit">⚙️ Config (rules)</a>';
echo '</div>';
echo '</form>';

echo '</div></div>';
