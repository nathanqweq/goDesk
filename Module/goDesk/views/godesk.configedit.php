<?php
(new CHtmlPage())
	->setTitle($data['title'])
	->show();

function h($s) { return htmlspecialchars((string)$s, ENT_QUOTES, 'UTF-8'); }

$def = $data['default'] ?? [];
$def_td = $def['topdesk'] ?? [];
$named_clients = $data['named_clients'] ?? [];
$rules = $data['rules'] ?? [];

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
if (!empty($data['error'])) {
	echo '<div class="gd-banner gd-err"><b>Erro:</b> '.h($data['error']).'</div>';
}
if (!empty($data['sync_status'])) {
	echo '<div class="gd-banner gd-ok">'.h($data['sync_status']).'</div>';
}
if (!empty($data['sync_error'])) {
	echo '<div class="gd-banner gd-err"><b>Sincronização:</b> '.h($data['sync_error']).'</div>';
}

echo '<form method="post" action="zabbix.php?action=godesk.config.edit">';
echo '<input type="hidden" name="save" value="1">';

echo '<div class="gd-card">';
echo '<h2>📦 Default</h2>';

echo '<div class="gd-row">';
echo '<div class="gd-field"><label>Client (fallback display)</label><input type="text" name="default[client]" value="'.h($def['client'] ?? '').'"></div>';
echo '<div class="gd-field"><label>Urgency</label><input type="text" name="default[urgency]" value="'.h($def['urgency'] ?? '').'"></div>';
echo '<div class="gd-field"><label>Impact</label><input type="text" name="default[impact]" value="'.h($def['impact'] ?? '').'"></div>';
echo '</div>';

echo '<div class="gd-row">';
echo '<div class="gd-field"><label>Priority</label><input type="text" name="default[priority]" value="'.h($def['priority'] ?? '').'"></div>';

$checked = !empty($def['autoclose']) ? 'checked' : '';
echo '<div class="gd-field gd-field-tight">
	<label>Autoclose</label>
	<div class="gd-check">
		<input type="checkbox" class="gd-autoclose" name="default[autoclose]" value="1" '.$checked.'>
		<span class="gd-muted">fechar automaticamente</span>
	</div>
</div>';
echo '</div>';

echo '<div class="gd-divider"></div>';
echo '<div class="gd-small-title">🎫 TopDesk</div>';

echo '<div class="gd-row">';
echo '<div class="gd-field"><label>contract</label><input type="text" name="default[topdesk][contract]" value="'.h($def_td['contract'] ?? '').'"></div>';
echo '<div class="gd-field"><label>operator</label><input type="text" name="default[topdesk][operator]" value="'.h($def_td['operator'] ?? '').'"></div>';
echo '<div class="gd-field"><label>oper_group</label><input type="text" name="default[topdesk][oper_group]" value="'.h($def_td['oper_group'] ?? '').'"></div>';
echo '</div>';

echo '<div class="gd-row">';
echo '<div class="gd-field"><label>main_caller</label><input type="text" name="default[topdesk][main_caller]" value="'.h($def_td['main_caller'] ?? '').'"></div>';
echo '<div class="gd-field"><label>secundary_caller</label><input type="text" name="default[topdesk][secundary_caller]" value="'.h($def_td['secundary_caller'] ?? '').'"></div>';
echo '<div class="gd-field"><label>sla</label><input type="text" class="gd-sla" name="default[topdesk][sla]" value="'.h($def_td['sla'] ?? '').'"></div>';
echo '</div>';

echo '<div class="gd-row">';
echo '<div class="gd-field"><label>category</label><input type="text" name="default[topdesk][category]" value="'.h($def_td['category'] ?? '').'"></div>';
echo '<div class="gd-field"><label>sub_category</label><input type="text" name="default[topdesk][sub_category]" value="'.h($def_td['sub_category'] ?? '').'"></div>';
echo '<div class="gd-field"><label>call_type</label><input type="text" name="default[topdesk][call_type]" value="'.h($def_td['call_type'] ?? '').'"></div>';
echo '</div>';

$def_send_more = !empty($def_td['send_more_info']) ? 'checked' : '';
$def_send_more_hidden = !empty($def_td['send_more_info']) ? '' : ' style="display:none"';
echo '<div class="gd-row">';
echo '<div class="gd-field gd-field-tight">
	<label>Sendmore info</label>
	<div class="gd-check">
		<input type="checkbox" class="gd-sendmore-toggle" name="default[topdesk][send_more_info]" value="1" '.$def_send_more.'>
		<span class="gd-muted">comentar após criar o chamado</span>
	</div>
</div>';
echo '</div>';
echo '<div class="gd-row gd-sendmore-box"'.$def_send_more_hidden.'>';
echo '<div class="gd-field"><label>Texto sendmore (puro)</label><textarea class="gd-sendmore-text" name="default[topdesk][more_info_text]" rows="4">'.h($def_td['more_info_text'] ?? '').'</textarea></div>';
echo '</div>';

$def_add_cresol = !empty($def_td['adicional_cresol']) ? 'checked' : '';
echo '<div class="gd-row">';
echo '<div class="gd-field gd-field-tight">
	<label>Adicional cresol</label>
	<div class="gd-check">
		<input type="checkbox" name="default[topdesk][adicional_cresol]" value="1" '.$def_add_cresol.'>
		<span class="gd-muted">enviar optionalFields1 no create</span>
	</div>
</div>';
echo '</div>';

$def_send_email = !empty($def_td['send_email']) ? 'checked' : '';
$def_send_email_hidden = !empty($def_td['send_email']) ? '' : ' style="display:none"';
echo '<div class="gd-row">';
echo '<div class="gd-field gd-field-tight">
	<label>Enviar email</label>
	<div class="gd-check">
		<input type="checkbox" class="gd-sendemail-toggle" name="default[topdesk][send_email]" value="1" '.$def_send_email.'>
		<span class="gd-muted">enviar email apos criar o chamado</span>
	</div>
</div>';
echo '</div>';
echo '<div class="gd-row gd-sendemail-box"'.$def_send_email_hidden.'>';
echo '<div class="gd-field"><label>Email para</label><input type="text" class="gd-email-to" name="default[topdesk][email_to]" value="'.h($def_td['email_to'] ?? '').'"></div>';
echo '<div class="gd-field"><label>Email copia</label><input type="text" class="gd-email-cc" name="default[topdesk][email_cc]" value="'.h($def_td['email_cc'] ?? '').'"></div>';
echo '</div>';

echo '</div>';

// ===================== CLIENTES (compartilham TopDesk entre várias rules) =====================
echo '<div class="gd-card">';
echo '<div class="gd-client-head">';
echo '<h2 style="margin:0">🏢 Clientes</h2>';
echo '<button class="gd-btn" type="button" data-gd-add-named-client>＋ Adicionar cliente</button>';
echo '</div>';
echo '<div class="gd-muted" style="margin-bottom:10px">Cadastre um cliente aqui pra não repetir o mesmo TopDesk em várias rules — cada rule abaixo escolhe um cliente e só precisa preencher o que for diferente (ex: more_info_text do circuito).</div>';

echo '<div class="gd-row"><div class="gd-field"><label>🔎 Buscar cliente</label><input type="text" id="gd-filter-named-clients" placeholder="Filtrar por nome do cliente..."></div></div>';

echo '<div id="gd-named-clients">';

$nidx = 0;
foreach ($named_clients as $nc) {
	$name = $nc['name'] ?? '';
	$td = $nc['topdesk'] ?? [];

	echo '<div class="gd-client-card gd-named-client" data-idx="'.$nidx.'">';

	echo '<div class="gd-client-head">';
	echo '<div class="gd-client-name">🏢 Cliente</div>';
	echo '<button class="gd-btn gd-btn-danger" type="button" data-gd-remove-named-client>Remover</button>';
	echo '</div>';

	echo '<div class="gd-row">';
	echo '<div class="gd-field"><label>Nome do cliente (chave do YAML)</label><input type="text" class="gd-named-client-name" name="named_clients['.$nidx.'][name]" value="'.h($name).'"></div>';
	echo '</div>';

	echo '<div class="gd-divider"></div>';
	echo '<div class="gd-small-title">🎫 TopDesk (compartilhado por todas as rules deste cliente)</div>';

	echo '<div class="gd-row">';
	echo '<div class="gd-field"><label>contract</label><input type="text" name="named_clients['.$nidx.'][topdesk][contract]" value="'.h($td['contract'] ?? '').'"></div>';
	echo '<div class="gd-field"><label>operator</label><input type="text" name="named_clients['.$nidx.'][topdesk][operator]" value="'.h($td['operator'] ?? '').'"></div>';
	echo '<div class="gd-field"><label>oper_group</label><input type="text" name="named_clients['.$nidx.'][topdesk][oper_group]" value="'.h($td['oper_group'] ?? '').'"></div>';
	echo '</div>';

	echo '<div class="gd-row">';
	echo '<div class="gd-field"><label>main_caller</label><input type="text" name="named_clients['.$nidx.'][topdesk][main_caller]" value="'.h($td['main_caller'] ?? '').'"></div>';
	echo '<div class="gd-field"><label>secundary_caller</label><input type="text" name="named_clients['.$nidx.'][topdesk][secundary_caller]" value="'.h($td['secundary_caller'] ?? '').'"></div>';
	echo '<div class="gd-field"><label>sla</label><input type="text" class="gd-sla" name="named_clients['.$nidx.'][topdesk][sla]" value="'.h($td['sla'] ?? '').'"></div>';
	echo '</div>';

	echo '<div class="gd-row">';
	echo '<div class="gd-field"><label>category</label><input type="text" name="named_clients['.$nidx.'][topdesk][category]" value="'.h($td['category'] ?? '').'"></div>';
	echo '<div class="gd-field"><label>sub_category</label><input type="text" name="named_clients['.$nidx.'][topdesk][sub_category]" value="'.h($td['sub_category'] ?? '').'"></div>';
	echo '<div class="gd-field"><label>call_type</label><input type="text" name="named_clients['.$nidx.'][topdesk][call_type]" value="'.h($td['call_type'] ?? '').'"></div>';
	echo '</div>';

	$nc_send_more = !empty($td['send_more_info']) ? 'checked' : '';
	$nc_send_more_hidden = !empty($td['send_more_info']) ? '' : ' style="display:none"';
	echo '<div class="gd-row">';
	echo '<div class="gd-field gd-field-tight">
		<label>Sendmore info</label>
		<div class="gd-check">
			<input type="checkbox" class="gd-sendmore-toggle" name="named_clients['.$nidx.'][topdesk][send_more_info]" value="1" '.$nc_send_more.'>
			<span class="gd-muted">comentar após criar o chamado</span>
		</div>
	</div>';
	echo '</div>';
	echo '<div class="gd-row gd-sendmore-box"'.$nc_send_more_hidden.'>';
	echo '<div class="gd-field"><label>Texto sendmore (padrão do cliente)</label><textarea class="gd-sendmore-text" name="named_clients['.$nidx.'][topdesk][more_info_text]" rows="4">'.h($td['more_info_text'] ?? '').'</textarea></div>';
	echo '</div>';

	$nc_add_cresol = !empty($td['adicional_cresol']) ? 'checked' : '';
	echo '<div class="gd-row">';
	echo '<div class="gd-field gd-field-tight">
		<label>Adicional cresol</label>
		<div class="gd-check">
			<input type="checkbox" name="named_clients['.$nidx.'][topdesk][adicional_cresol]" value="1" '.$nc_add_cresol.'>
			<span class="gd-muted">enviar optionalFields1 no create</span>
		</div>
	</div>';
	echo '</div>';

	$nc_send_email = !empty($td['send_email']) ? 'checked' : '';
	$nc_send_email_hidden = !empty($td['send_email']) ? '' : ' style="display:none"';
	echo '<div class="gd-row">';
	echo '<div class="gd-field gd-field-tight">
		<label>Enviar email</label>
		<div class="gd-check">
			<input type="checkbox" class="gd-sendemail-toggle" name="named_clients['.$nidx.'][topdesk][send_email]" value="1" '.$nc_send_email.'>
			<span class="gd-muted">enviar email apos criar o chamado</span>
		</div>
	</div>';
	echo '</div>';
	echo '<div class="gd-row gd-sendemail-box"'.$nc_send_email_hidden.'>';
	echo '<div class="gd-field"><label>Email para</label><input type="text" class="gd-email-to" name="named_clients['.$nidx.'][topdesk][email_to]" value="'.h($td['email_to'] ?? '').'"></div>';
	echo '<div class="gd-field"><label>Email copia</label><input type="text" class="gd-email-cc" name="named_clients['.$nidx.'][topdesk][email_cc]" value="'.h($td['email_cc'] ?? '').'"></div>';
	echo '</div>';

	echo '</div>';

	$nidx++;
}

echo '</div>';
echo '</div>';

// ===================== RULES =====================
echo '<div class="gd-card">';
echo '<div class="gd-client-head">';
echo '<h2 style="margin:0">👥 Rules</h2>';
echo '<button class="gd-btn" type="button" data-gd-add>＋ Adicionar rule</button>';
echo '</div>';

echo '<div class="gd-row"><div class="gd-field"><label>🔎 Buscar rule/cliente</label><input type="text" id="gd-filter-rules" placeholder="Filtrar por rule_name ou cliente..."></div></div>';

echo '<div id="gd-clients">';

$idx = 0;
foreach ($rules as $c) {
	$rule_name = $c['rule_name'] ?? '';
	$client_name = $c['client'] ?? '';
	$td = $c['topdesk'] ?? [];
	$autoclose = !empty($c['autoclose']) ? 'checked' : '';

	echo '<div class="gd-client-card gd-client" data-idx="'.$idx.'">';

	echo '<div class="gd-client-head">';
	echo '<div class="gd-client-name">🧩 Rule</div>';
	echo '<button class="gd-btn gd-btn-danger" type="button" data-gd-remove>Remover</button>';
	echo '</div>';

	echo '<div class="gd-row">';
	echo '<div class="gd-field"><label>Rule name (chave do YAML)</label><input type="text" name="clients['.$idx.'][rule_name]" value="'.h($rule_name).'"></div>';

	echo '<div class="gd-field"><label>Cliente</label><select class="gd-client-select" name="clients['.$idx.'][client]">';
	echo '<option value="">— nenhum / customizado —</option>';
	$seen_current = false;
	foreach ($named_clients as $nc) {
		$nc_name = $nc['name'] ?? '';
		if ($nc_name === '') {
			continue;
		}
		$sel = ($nc_name === $client_name) ? ' selected' : '';
		if ($sel !== '') { $seen_current = true; }
		echo '<option value="'.h($nc_name).'"'.$sel.'>'.h($nc_name).'</option>';
	}
	if ($client_name !== '' && !$seen_current) {
		// valor atual não bate com nenhum cliente cadastrado (ex: dado antigo) — mantém pra não perder
		echo '<option value="'.h($client_name).'" selected>'.h($client_name).' (não cadastrado)</option>';
	}
	echo '</select></div>';

	echo '<div class="gd-field"><label>Urgency</label><input type="text" name="clients['.$idx.'][urgency]" value="'.h($c['urgency'] ?? '').'"></div>';
	echo '</div>';

	echo '<div class="gd-row">';
	echo '<div class="gd-field"><label>Impact</label><input type="text" name="clients['.$idx.'][impact]" value="'.h($c['impact'] ?? '').'"></div>';
	echo '<div class="gd-field"><label>Priority</label><input type="text" name="clients['.$idx.'][priority]" value="'.h($c['priority'] ?? '').'"></div>';
	echo '<div class="gd-field gd-field-tight">
		<label>Autoclose</label>
		<div class="gd-check">
			<input type="checkbox" class="gd-autoclose" name="clients['.$idx.'][autoclose]" value="1" '.$autoclose.'>
		</div>
	</div>';
	echo '</div>';

	echo '<div class="gd-divider"></div>';
	echo '<div class="gd-small-title">🎫 TopDesk <span class="gd-muted">(vazio = herda do cliente escolhido acima; preenchido = sobrescreve só nesta rule)</span></div>';

	echo '<div class="gd-row">';
	echo '<div class="gd-field"><label>contract</label><input type="text" name="clients['.$idx.'][topdesk][contract]" value="'.h($td['contract'] ?? '').'"></div>';
	echo '<div class="gd-field"><label>operator</label><input type="text" name="clients['.$idx.'][topdesk][operator]" value="'.h($td['operator'] ?? '').'"></div>';
	echo '<div class="gd-field"><label>oper_group</label><input type="text" name="clients['.$idx.'][topdesk][oper_group]" value="'.h($td['oper_group'] ?? '').'"></div>';
	echo '</div>';

	echo '<div class="gd-row">';
	echo '<div class="gd-field"><label>main_caller</label><input type="text" name="clients['.$idx.'][topdesk][main_caller]" value="'.h($td['main_caller'] ?? '').'"></div>';
	echo '<div class="gd-field"><label>secundary_caller</label><input type="text" name="clients['.$idx.'][topdesk][secundary_caller]" value="'.h($td['secundary_caller'] ?? '').'"></div>';
	echo '<div class="gd-field"><label>sla</label><input type="text" class="gd-sla" name="clients['.$idx.'][topdesk][sla]" value="'.h($td['sla'] ?? '').'"></div>';
	echo '</div>';

	echo '<div class="gd-row">';
	echo '<div class="gd-field"><label>category</label><input type="text" name="clients['.$idx.'][topdesk][category]" value="'.h($td['category'] ?? '').'"></div>';
	echo '<div class="gd-field"><label>sub_category</label><input type="text" name="clients['.$idx.'][topdesk][sub_category]" value="'.h($td['sub_category'] ?? '').'"></div>';
	echo '<div class="gd-field"><label>call_type</label><input type="text" name="clients['.$idx.'][topdesk][call_type]" value="'.h($td['call_type'] ?? '').'"></div>';
	echo '</div>';

	$send_more = !empty($td['send_more_info']) ? 'checked' : '';
	$send_more_hidden = !empty($td['send_more_info']) ? '' : ' style="display:none"';
	echo '<div class="gd-row">';
	echo '<div class="gd-field gd-field-tight">
		<label>Sendmore info</label>
		<div class="gd-check">
			<input type="checkbox" class="gd-sendmore-toggle" name="clients['.$idx.'][topdesk][send_more_info]" value="1" '.$send_more.'>
			<span class="gd-muted">comentar após criar o chamado (só se não tiver cliente escolhido — em rule com cliente, quem manda é o cliente)</span>
		</div>
	</div>';
	echo '</div>';
	echo '<div class="gd-row gd-sendmore-box"'.$send_more_hidden.'>';
	echo '<div class="gd-field"><label>Texto sendmore (puro)</label><textarea class="gd-sendmore-text" name="clients['.$idx.'][topdesk][more_info_text]" rows="4">'.h($td['more_info_text'] ?? '').'</textarea></div>';
	echo '</div>';

	$add_cresol = !empty($td['adicional_cresol']) ? 'checked' : '';
	echo '<div class="gd-row">';
	echo '<div class="gd-field gd-field-tight">
		<label>Adicional cresol</label>
		<div class="gd-check">
			<input type="checkbox" name="clients['.$idx.'][topdesk][adicional_cresol]" value="1" '.$add_cresol.'>
			<span class="gd-muted">enviar optionalFields1 no create (só se não tiver cliente escolhido)</span>
		</div>
	</div>';
	echo '</div>';

	$send_email = !empty($td['send_email']) ? 'checked' : '';
	$send_email_hidden = !empty($td['send_email']) ? '' : ' style="display:none"';
	echo '<div class="gd-row">';
	echo '<div class="gd-field gd-field-tight">
		<label>Enviar email</label>
		<div class="gd-check">
			<input type="checkbox" class="gd-sendemail-toggle" name="clients['.$idx.'][topdesk][send_email]" value="1" '.$send_email.'>
			<span class="gd-muted">enviar email apos criar o chamado (só se não tiver cliente escolhido)</span>
		</div>
	</div>';
	echo '</div>';
	echo '<div class="gd-row gd-sendemail-box"'.$send_email_hidden.'>';
	echo '<div class="gd-field"><label>Email para</label><input type="text" class="gd-email-to" name="clients['.$idx.'][topdesk][email_to]" value="'.h($td['email_to'] ?? '').'"></div>';
	echo '<div class="gd-field"><label>Email copia</label><input type="text" class="gd-email-cc" name="clients['.$idx.'][topdesk][email_cc]" value="'.h($td['email_cc'] ?? '').'"></div>';
	echo '</div>';

	echo '</div>';

	$idx++;
}

echo '</div>';

echo '<div class="gd-actions">';
echo '<button class="gd-btn gd-btn-primary" type="submit">💾 Salvar</button>';
echo '<a class="gd-btn" href="zabbix.php?action=godesk.config.edit">↩ Recarregar</a>';
echo '<a class="gd-btn" href="zabbix.php?action=godesk.config.view">👁️ Visualizar</a>';
echo '</div>';

echo '</div>';

echo '</form>';

echo '</div></div>';
