<?php

(new CHtmlPage())
	->setTitle($data['title'])
	->show();

$def = $data['default'] ?? [];
$def_tags = $def['tags'] ?? [];
$clients = $data['clients'] ?? [];

function h($s) { return htmlspecialchars((string)$s, ENT_QUOTES, 'UTF-8'); }

echo '<div class="gd-wrap">';
echo '<h1>⚙️ goDesk — Editor de Config</h1>';
echo '<div class="gd-small"><b>Arquivo:</b> '.h($data['path']).'</div>';

if (!empty($data['status'])) {
	echo '<div class="gd-banner gd-ok">'.h($data['status']).'</div>';
	if (!empty($data['backup'])) {
		echo '<div class="gd-small"><b>Backup:</b> '.h($data['backup']).'</div>';
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
echo '<div class="gd-field" style="min-width:180px;flex:0 0 auto">
	<label>Autoclose</label>
	<div><input type="checkbox" name="default[autoclose]" value="1" '.$checked.'> <span class="gd-small">fechar automaticamente</span></div>
</div>';

echo '</div>';

echo '<h3 style="margin:14px 0 8px 0">🏷️ Tags</h3>';
echo '<div class="gd-row">';
echo '<div class="gd-field"><label>contract</label><input type="text" name="default[tags][contract]" value="'.h($def_tags['contract'] ?? '').'"></div>';
echo '<div class="gd-field"><label>oper_group</label><input type="text" name="default[tags][oper_group]" value="'.h($def_tags['oper_group'] ?? '').'"></div>';
echo '<div class="gd-field"><label>main_caller</label><input type="text" name="default[tags][main_caller]" value="'.h($def_tags['main_caller'] ?? '').'"></div>';
echo '<div class="gd-field"><label>secundary_caller</label><input type="text" name="default[tags][secundary_caller]" value="'.h($def_tags['secundary_caller'] ?? '').'"></div>';
echo '</div>';

echo '</div>';

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
	$tags = $c['tags'] ?? [];
	$autoclose = !empty($c['autoclose']) ? 'checked' : '';

	echo '<div class="gd-card gd-client" data-idx="'.$idx.'">';
	echo '<div class="gd-client-head">';
	echo '<div class="gd-small"><b>Cliente</b></div>';
	echo '<button class="gd-btn" type="button" onclick="gdRemoveClient(this)">Remover</button>';
	echo '</div>';

	echo '<div class="gd-row">';
	echo '<div class="gd-field"><label>Nome</label><input type="text" name="clients['.$idx.'][name]" value="'.h($name).'"></div>';
	echo '<div class="gd-field"><label>Urgency</label><input type="text" name="clients['.$idx.'][urgency]" value="'.h($c['urgency'] ?? '').'"></div>';
	echo '<div class="gd-field"><label>Impact</label><input type="text" name="clients['.$idx.'][impact]" value="'.h($c['impact'] ?? '').'"></div>';
	echo '<div class="gd-field" style="min-width:180px;flex:0 0 auto">
		<label>Autoclose</label>
		<div><input type="checkbox" name="clients['.$idx.'][autoclose]" value="1" '.$autoclose.'></div>
	</div>';
	echo '</div>';

	echo '<h3 style="margin:14px 0 8px 0">🏷️ Tags</h3>';
	echo '<div class="gd-row">';
	echo '<div class="gd-field"><label>contract</label><input type="text" name="clients['.$idx.'][tags][contract]" value="'.h($tags['contract'] ?? '').'"></div>';
	echo '<div class="gd-field"><label>oper_group</label><input type="text" name="clients['.$idx.'][tags][oper_group]" value="'.h($tags['oper_group'] ?? '').'"></div>';
	echo '<div class="gd-field"><label>main_caller</label><input type="text" name="clients['.$idx.'][tags][main_caller]" value="'.h($tags['main_caller'] ?? '').'"></div>';
	echo '<div class="gd-field"><label>secundary_caller</label><input type="text" name="clients['.$idx.'][tags][secundary_caller]" value="'.h($tags['secundary_caller'] ?? '').'"></div>';
	echo '</div>';

	echo '</div>';

	$idx++;
}

echo '</div>'; // gd-clients
echo '</div>'; // card clients

echo '<div class="gd-actions">';
echo '<button class="gd-btn" type="submit">💾 Salvar</button>';
echo '<a class="gd-btn" href="zabbix.php?action=godesk.config.edit" style="text-decoration:none;color:inherit">↩ Recarregar</a>';
echo '</div>';

echo '</form>';
echo '</div>';

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
	<div class="gd-card gd-client" data-idx="${i}">
		<div class="gd-client-head">
			<div class="gd-small"><b>Cliente</b></div>
			<button class="gd-btn" type="button" onclick="gdRemoveClient(this)">Remover</button>
		</div>

		<div class="gd-row">
			<div class="gd-field"><label>Nome</label><input type="text" name="clients[${i}][name]" value=""></div>
			<div class="gd-field"><label>Urgency</label><input type="text" name="clients[${i}][urgency]" value=""></div>
			<div class="gd-field"><label>Impact</label><input type="text" name="clients[${i}][impact]" value=""></div>
			<div class="gd-field" style="min-width:180px;flex:0 0 auto">
				<label>Autoclose</label>
				<div><input type="checkbox" name="clients[${i}][autoclose]" value="1"></div>
			</div>
		</div>

		<h3 style="margin:14px 0 8px 0">🏷️ Tags</h3>
		<div class="gd-row">
			<div class="gd-field"><label>contract</label><input type="text" name="clients[${i}][tags][contract]" value=""></div>
			<div class="gd-field"><label>oper_group</label><input type="text" name="clients[${i}][tags][oper_group]" value=""></div>
			<div class="gd-field"><label>main_caller</label><input type="text" name="clients[${i}][tags][main_caller]" value=""></div>
			<div class="gd-field"><label>secundary_caller</label><input type="text" name="clients[${i}][tags][secundary_caller]" value=""></div>
		</div>
	</div>`;
	host.insertAdjacentHTML("beforeend", html);
}
</script>';
