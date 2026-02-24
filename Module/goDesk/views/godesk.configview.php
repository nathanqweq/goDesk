<?php
(new CHtmlPage())
	->setTitle($data['title'])
	->show();

function h($s) { return htmlspecialchars((string)$s, ENT_QUOTES, 'UTF-8'); }

$config = $data['config'] ?? [];
$def = $config['default'] ?? [];
$def_td = $def['topdesk'] ?? [];
$clients = $config['clients'] ?? [];

echo '<div class="godesk-module">';
echo '<div class="gd-wrap">';

echo '<div class="gd-header">';
echo '<div class="gd-title">'.h($data['title']).'</div>';
echo '<div class="gd-subtitle"><b>Arquivo:</b> '.h($data['path'] ?? '').'</div>';
echo '</div>';

if (isset($config['_error'])) {
	echo '<div class="gd-banner gd-err"><b>Erro:</b> '.h($config['_error']).'</div>';
	echo '</div></div>';
	return;
}

echo '<div class="gd-card">';
echo '<h2>📦 Default</h2>';

echo '<div class="gd-row">';
echo '<div class="gd-kv"><span class="gd-k">Client (fallback)</span><span class="gd-v">'.h($def['client'] ?? '').'</span></div>';
echo '<div class="gd-kv"><span class="gd-k">Urgency</span><span class="gd-v">'.h($def['urgency'] ?? '').'</span></div>';
echo '<div class="gd-kv"><span class="gd-k">Impact</span><span class="gd-v">'.h($def['impact'] ?? '').'</span></div>';
echo '<div class="gd-kv"><span class="gd-k">Priority</span><span class="gd-v">'.h($def['priority'] ?? '').'</span></div>';

$auto = !empty($def['autoclose']) ? '<span class="gd-pill gd-true">true</span>' : '<span class="gd-pill gd-false">false</span>';
echo '<div class="gd-kv"><span class="gd-k">Autoclose</span><span class="gd-v">'.$auto.'</span></div>';
echo '</div>';

echo '<div class="gd-divider"></div>';
echo '<div class="gd-small-title">🎫 TopDesk</div>';
echo '<div class="gd-tags">';
foreach (['contract','operator','oper_group','main_caller','secundary_caller','sla','category','sub_category','call_type'] as $k) {
	$v = $def_td[$k] ?? '';
	echo '<span class="gd-tag">'.h($k).': '.h($v).'</span>';
}
echo '<span class="gd-tag">send_more_info: '.(!empty($def_td['send_more_info']) ? 'true' : 'false').'</span>';
echo '<span class="gd-tag">send_email: '.(!empty($def_td['send_email']) ? 'true' : 'false').'</span>';
echo '<span class="gd-tag">email_to: '.h($def_td['email_to'] ?? '').'</span>';
echo '<span class="gd-tag">email_cc: '.h($def_td['email_cc'] ?? '').'</span>';
echo '</div>';
if (!empty($def_td['more_info_text'])) {
	echo '<div class="gd-row" style="margin-top:10px;">';
	echo '<div class="gd-kv"><span class="gd-k">more_info_text</span><span class="gd-v">'.nl2br(h($def_td['more_info_text'])).'</span></div>';
	echo '</div>';
}
echo '</div>';

echo '<div class="gd-card">';
echo '<h2>👥 Rules</h2>';

if (!is_array($clients) || count($clients) === 0) {
	echo '<div class="gd-muted">Nenhuma rule cadastrada.</div>';
}
else {
	foreach ($clients as $rule_name => $c) {
		$td = $c['topdesk'] ?? [];

		echo '<div class="gd-client-card">';
		echo '<div class="gd-client-head">';
		echo '<div class="gd-client-name">🧩 Rule: '.h($rule_name).'</div>';

		$c_auto = !empty($c['autoclose']) ? '<span class="gd-pill gd-true">autoclose</span>' : '<span class="gd-pill gd-false">manual</span>';
		echo '<div>'.$c_auto.'</div>';
		echo '</div>';

		$client_name = (string)($c['client'] ?? '');
		if ($client_name !== '') {
			echo '<div class="gd-muted"><b>Client:</b> '.h($client_name).'</div>';
		}

		echo '<div class="gd-row" style="margin-top:10px;">';
		echo '<div class="gd-kv"><span class="gd-k">Urgency</span><span class="gd-v">'.h($c['urgency'] ?? '').'</span></div>';
		echo '<div class="gd-kv"><span class="gd-k">Impact</span><span class="gd-v">'.h($c['impact'] ?? '').'</span></div>';
		echo '<div class="gd-kv"><span class="gd-k">Priority</span><span class="gd-v">'.h($c['priority'] ?? '').'</span></div>';
		echo '</div>';

		echo '<div class="gd-small-title" style="margin-top:10px;">🎫 TopDesk</div>';
		echo '<div class="gd-tags">';
		foreach (['contract','operator','oper_group','main_caller','secundary_caller','sla','category','sub_category','call_type'] as $k) {
			$v = $td[$k] ?? '';
			echo '<span class="gd-tag">'.h($k).': '.h($v).'</span>';
		}
		echo '<span class="gd-tag">send_more_info: '.(!empty($td['send_more_info']) ? 'true' : 'false').'</span>';
		echo '<span class="gd-tag">send_email: '.(!empty($td['send_email']) ? 'true' : 'false').'</span>';
		echo '<span class="gd-tag">email_to: '.h($td['email_to'] ?? '').'</span>';
		echo '<span class="gd-tag">email_cc: '.h($td['email_cc'] ?? '').'</span>';
		echo '</div>';
		if (!empty($td['more_info_text'])) {
			echo '<div class="gd-row" style="margin-top:10px;">';
			echo '<div class="gd-kv"><span class="gd-k">more_info_text</span><span class="gd-v">'.nl2br(h($td['more_info_text'])).'</span></div>';
			echo '</div>';
		}

		echo '</div>';
	}
}

echo '</div>';

echo '</div></div>';
