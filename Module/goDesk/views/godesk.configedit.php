<?php
(new CHtmlPage())
	->setTitle($data['title'])
	->show();

function h($s) { return htmlspecialchars((string)$s, ENT_QUOTES, 'UTF-8'); }

$def = $data['default'] ?? [];
$def_td = $def['topdesk'] ?? [];
$clients = $data['clients'] ?? [];

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

echo '<form method="post" action="zabbix.php?action=godesk.config.edit">';
echo '<input type="hidden" name="save" value="1">';

//
// DEFAULT
//
echo '<div class="gd-card">';
echo '<h2>📦 Default</h2>';

echo '<div class="gd-row">';
echo '<div class="gd-field"><label>Urgency</label><input type="text" name="default[urgency]" value="'.h($def['urgency'] ?? '').'"></div>';
echo '<div class="gd-field"><label>Impact</label><input type="text" name="default[impact]" value="'.h($def['impact'] ?? '').'"></div>';

$checked = !empty($def['autoclose']) ? 'checked' : '';
echo '<div class="gd-field gd-field-tight">
	<label>Autoclose</label>
	<div class="gd-check">
		<input type="checkbox" name="default[autoclose]" value="1" '.$checked.'>
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
echo '<div class="gd-field"><label>sla</label><input type="text" name="default[topdesk][sla]" value="'.h($def_td['sla'] ?? '').'"></div>';
echo '</div>';

echo '<div class="gd-row">';
echo '<div class="gd-field"><label>category</label><input type="text" name="default[topdesk][category]" value="'.h($def_td['category'] ?? '').'"></div>';
echo '<div class="gd-field"><label>sub_category</label><input type="text" name="default[topdesk][sub_category]" value="'.h($def_td['sub_category'] ?? '').'"></div>';
echo '<div class="gd-field"><label>call_type</label><input type="text" name="default[topdesk][call_type]" value="'.h($def_td['call_type'] ?? '').'"></div>';
echo '</div>';

echo '</div>'; // default card

//
// CLIENTS
//
echo '<div class="gd-card">';
echo '<div class="gd-client-head">';
echo '<h2 style="margin:0">👥 Clients</h2>';
echo '<button class="gd-btn" type="button" onclick="gdAddClient()">＋ Adicionar cliente</button>';
echo '</div>';

echo '<div id="gd-clients">';

$idx = 0;
foreach ($clients as $c) {
	$name = $c['name'] ?? '';
	$td = $c['topdesk'] ?? [];
	$autoclose = !empty($c['autoclose']) ? 'checked' : '';

	echo '<div class="gd-client-card gd-client" data-idx="'.$idx.'">';

	echo '<div class="gd-client-head">';
	echo '<div class="gd-client-name">🏢 Cliente</div>';
	echo '<button class="gd-btn gd-btn-danger" type="button" onclick="gdRemoveClient(this)">Remover</button>';
	echo '</div>';

	echo '<div class="gd-row">';
	echo '<div class="gd-field"><label>Nome</label><input type="text" name="clients['.$idx.'][name]" value="'.h($name).'"></div>';
	echo '<div class="gd-field"><label>Urgency</label><input type="text" name="clients['.$idx.'][urgency]" value="'.h($c['urgency'] ?? '').'"></div>';
	echo '<div class="gd-field"><label>Impact</label><input type="text" name="clients['.$idx.'][impact]" value="'.h($c['impact'] ?? '').'"></div>';
	echo '<div class="gd-field gd-field-tight">
		<label>Autoclose</label>
		<div class="gd-check"><input type="checkbox" name="clients['.$idx.'][autoclose]" value="1" '.$autoclose.'></div>
	</div>';
	echo '</div>';

	echo '<div class="gd-divider"></div>';
	echo '<div class="gd-small-title">🎫 TopDesk</div>';

	echo '<div class="gd-row">';
	echo '<div class="gd-field"><label>contract</label><input type="text" name="clients['.$idx.'][topdesk][contract]" value="'.h($td['contract'] ?? '').'"></div>';
	echo '<div class="gd-field"><label>operator</label><input type="text" name="clients['.$idx.'][topdesk][operator]" value="'.h($td['operator'] ?? '').'"></div>';
	echo '<div class="gd-field"><label>oper_group</label><input type="text" name="clients['.$idx.'][topdesk][oper_group]" value="'.h($td['oper_group'] ?? '').'"></div>';
	echo '</div>';

	echo '<div class="gd-row">';
	echo '<div class="gd-field"><label>main_caller</label><input type="text" name="clients['.$idx.'][topdesk][main_caller]" value="'.h($td['main_caller'] ?? '').'"></div>';
	echo '<div class="gd-field"><label>secundary_caller</label><input type="text" name="clients['.$idx.'][topdesk][secundary_caller]" value="'.h($td['secundary_caller'] ?? '').'"></div>';
	echo '<div class="gd-field"><label>sla</label><input type="text" name="clients['.$idx.'][topdesk][sla]" value="'.h($td['sla'] ?? '').'"></div>';
	echo '</div>';

	echo '<div class="gd-row">';
	echo '<div class="gd-field"><label>category</label><input type="text" name="clients['.$idx.'][topdesk][category]" value="'.h($td['category'] ?? '').'"></div>';
	echo '<div class="gd-field"><label>sub_category</label><input type="text" name="clients['.$idx.'][topdesk][sub_category]" value="'.h($td['sub_category'] ?? '').'"></div>';
	echo '<div class="gd-field"><label>call_type</label><input type="text" name="clients['.$idx.'][topdesk][call_type]" value="'.h($td['call_type'] ?? '').'"></div>';
	echo '</div>';

	echo '</div>'; // client card

	$idx++;
}

echo '</div>'; // gd-clients

echo '<div class="gd-actions">';
echo '<button class="gd-btn gd-btn-primary" type="submit">💾 Salvar</button>';
echo '<a class="gd-btn" href="zabbix.php?action=godesk.config.edit" style="text-decoration:none;">↩ Recarregar</a>';
echo '<a class="gd-btn" href="zabbix.php?action=godesk.config.view" style="text-decoration:none;">👁️ Visualizar</a>';
echo '</div>';

echo '</div>'; // card clients

echo '</form>';

echo '</div></div>'; // wrap + module

echo '<script>
let gdClientIdx = '.(int)$idx.';

function gdRemoveClient(btn){
	const card = btn.closest(".gd-client");
	if(card){ card.remove(); }
}

function gdAddClient(){
	const host = document.getElementById("gd-clients");
	const i = gdClientIdx++;

	const html = `
	<div class="gd-client-card gd-client" data-idx="${i}">
		<div class="gd-client-head">
			<div class="gd-client-name">🏢 Cliente</div>
			<button class="gd-btn gd-btn-danger" type="button" onclick="gdRemoveClient(this)">Remover</button>
		</div>

		<div class="gd-row">
			<div class="gd-field"><label>Nome</label><input type="text" name="clients[${i}][name]" value=""></div>
			<div class="gd-field"><label>Urgency</label><input type="text" name="clients[${i}][urgency]" value=""></div>
			<div class="gd-field"><label>Impact</label><input type="text" name="clients[${i}][impact]" value=""></div>
			<div class="gd-field gd-field-tight">
				<label>Autoclose</label>
				<div class="gd-check"><input type="checkbox" name="clients[${i}][autoclose]" value="1"></div>
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
			<div class="gd-field"><label>sla</label><input type="text" name="clients[${i}][topdesk][sla]" value=""></div>
		</div>

		<div class="gd-row">
			<div class="gd-field"><label>category</label><input type="text" name="clients[${i}][topdesk][category]" value=""></div>
			<div class="gd-field"><label>sub_category</label><input type="text" name="clients[${i}][topdesk][sub_category]" value=""></div>
			<div class="gd-field"><label>call_type</label><input type="text" name="clients[${i}][topdesk][call_type]" value=""></div>
		</div>
	</div>`;
	host.insertAdjacentHTML("beforeend", html);
}
</script>';
