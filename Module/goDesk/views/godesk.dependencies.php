<?php
(new CHtmlPage())
	->setTitle($data['title'])
	->show();

function h($s) { return htmlspecialchars((string)$s, ENT_QUOTES, 'UTF-8'); }

$deps = $data['deps'] ?? [];
$hosts = $data['hosts'] ?? [];

echo '<div class="godesk-module">';
echo '<div class="gd-wrap">';

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
echo '<h2>🔗 Como funciona</h2>';
echo '<div class="gd-muted">';
echo 'Quando um alerta de uma rule com <b>dependency</b> ligada chega para um host cadastrado aqui, o goDesk consulta os problemas ativos dos hosts pai no Zabbix. ';
echo 'Se o pai estiver em alerta, <b>o chamado não é aberto</b> e o goDesk comenta no evento do Zabbix (sem reconhecer o evento, então um escalonamento da Action pode reenviar o alerta depois).<br>';
echo '<b>Modo qualquer</b>: basta um pai em alerta. <b>Modo todos</b>: só segura se todos os pais estiverem em alerta (links/switches redundantes).<br>';
echo '<b>Evento a observar</b>: nome do problema no pai, aceita <code>*</code> como curinga. Vazio = qualquer problema do pai conta.';
echo '</div>';
echo '<div class="gd-actions">';
echo '<button class="gd-btn" type="button" data-gd-deps-export>⬇ Exportar</button>';
echo '<button class="gd-btn" type="button" data-gd-deps-import>⬆ Importar</button>';
echo '<input type="file" id="gd-deps-import-file" accept=".yaml,.yml,text/yaml" style="display:none">';
echo '</div>';
echo '</div>';

// conteúdo salvo atual (o que o Exportar baixa — não inclui edições não salvas)
echo '<textarea id="gd-deps-export-yaml" style="display:none">'.h($data['raw'] ?? '').'</textarea>';

echo '<form id="gd-deps-import-form" method="post" action="zabbix.php?action=godesk.dependencies.edit" style="display:none">';
echo '<input type="hidden" name="import" value="1">';
echo '<textarea name="import_yaml"></textarea>';
echo '</form>';

echo '<datalist id="gd-hosts-list">';
foreach ($hosts as $hh) {
	$label = ($hh['name'] ?? '') !== ($hh['host'] ?? '') ? $hh['name'] : '';
	echo '<option value="'.h($hh['host'] ?? '').'"'.($label !== '' ? ' label="'.h($label).'"' : '').'></option>';
}
echo '</datalist>';

echo '<form method="post" action="zabbix.php?action=godesk.dependencies.edit">';
echo '<input type="hidden" name="save" value="1">';

echo '<div class="gd-card">';
echo '<div class="gd-client-head">';
echo '<h2>🖥️ Hosts dependentes</h2>';
echo '<button class="gd-btn gd-btn-primary" type="button" data-gd-dep-add>+ Adicionar host dependente</button>';
echo '</div>';
echo '<div class="gd-field" style="margin-top:10px;"><input type="text" id="gd-filter-deps" placeholder="🔎 Filtrar por host dependente ou pai..."></div>';

echo '<div id="gd-deps" data-next-idx="'.count($deps).'">';

foreach ($deps as $i => $d) {
	$parents = $d['parents'] ?? [];

	echo '<div class="gd-client-card gd-dep" data-idx="'.$i.'" data-next-parent="'.count($parents).'">';

	echo '<div class="gd-client-head">';
	echo '<div class="gd-client-name">🖥️ Host dependente</div>';
	echo '<button class="gd-btn gd-btn-danger" type="button" data-gd-dep-remove>Remover</button>';
	echo '</div>';

	echo '<div class="gd-row">';
	echo '<div class="gd-field"><label>Host (nome técnico)</label><input type="text" class="gd-dep-host" list="gd-hosts-list" autocomplete="off" name="deps['.$i.'][host]" value="'.h($d['host'] ?? '').'">';
	if (($d['host'] ?? '') !== '' && ($d['hostid'] ?? '') === '') {
		echo '<span class="gd-muted gd-dep-missing">⚠ não encontrado neste Zabbix</span>';
	}
	echo '</div>';
	echo '<div class="gd-field gd-field-tight"><label>Modo</label><select name="deps['.$i.'][mode]">';
	echo '<option value="any"'.(($d['mode'] ?? 'any') === 'any' ? ' selected' : '').'>Qualquer pai em alerta</option>';
	echo '<option value="all"'.(($d['mode'] ?? '') === 'all' ? ' selected' : '').'>Todos os pais em alerta</option>';
	echo '</select></div>';
	echo '</div>';

	echo '<div class="gd-small-title" style="margin-top:12px;">Depende de</div>';
	echo '<div class="gd-dep-parents">';
	foreach ($parents as $j => $p) {
		$ev_list = 'gd-ev-'.$i.'-'.$j;
		echo '<div class="gd-row gd-dep-parent">';
		echo '<div class="gd-field"><label>Host pai</label><input type="text" class="gd-dep-parent-host" list="gd-hosts-list" autocomplete="off" name="deps['.$i.'][parents]['.$j.'][host]" value="'.h($p['host'] ?? '').'">';
		if (($p['hostid'] ?? '') === '') {
			echo '<span class="gd-muted gd-dep-missing">⚠ não encontrado neste Zabbix</span>';
		}
		echo '</div>';
		echo '<div class="gd-field"><label>Evento a observar <span class="gd-muted">(vazio = qualquer)</span></label><input type="text" class="gd-dep-parent-event" list="'.$ev_list.'" autocomplete="off" name="deps['.$i.'][parents]['.$j.'][event]" value="'.h($p['event'] ?? '').'" placeholder="ex: Unavailable by ICMP ping, *Interface*down*"><datalist id="'.$ev_list.'"></datalist></div>';
		echo '<div class="gd-field gd-field-tight" style="min-width:auto;"><button class="gd-btn gd-btn-danger" type="button" data-gd-dep-parent-remove>✕</button></div>';
		echo '</div>';
	}
	echo '</div>';
	echo '<div class="gd-actions"><button class="gd-btn" type="button" data-gd-dep-parent-add>+ Adicionar pai</button></div>';

	echo '</div>';
}

echo '</div>';
echo '</div>';

echo '<div class="gd-actions">';
echo '<button class="gd-btn gd-btn-primary" type="submit">💾 Salvar</button>';
echo '<a class="gd-btn" href="zabbix.php?action=godesk.dependencies.edit">↩ Recarregar</a>';
echo '<a class="gd-btn" href="zabbix.php?action=godesk.config.edit">⚙️ Config</a>';
echo '</div>';

echo '</form>';

echo '</div></div>';
