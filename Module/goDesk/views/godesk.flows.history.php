<?php
/**
 * View: godesk.flows.history — histórico das execuções de fluxo.
 */
(new CHtmlPage())
	->setTitle($data['title'])
	->show();

function h($s) { return htmlspecialchars((string)$s, ENT_QUOTES, 'UTF-8'); }

$f = $data['filters'] ?? [];
$entries = $data['entries'] ?? [];
$port_label = ['true' => 'sim', 'false' => 'não', 'default' => 'outro', 'found' => 'achou', 'not_found' => 'não achou', 'else' => 'senão', 'ok' => 'dentro', 'over' => 'passou', 'next' => ''];
$field_label = [
	'instrucao' => 'Instrução', 'urgency' => 'Urgência', 'impact' => 'Impacto', 'priority' => 'Prioridade',
	'oper_group' => 'Grupo operador', 'operator' => 'Operador', 'contract' => 'Contrato'
];

echo '<div class="godesk-module">';
echo '<div class="gd-wrap gd-wrap-wide">';

echo '<div class="gd-header">';
echo '<div class="gd-title">'.h($data['title']).'</div>';
echo '<div class="gd-subtitle">Cada alerta que passou por um fluxo (ativo ou shadow), mais recentes primeiro. <a href="zabbix.php?action=godesk.flows.edit">← Fluxos</a></div>';
echo '</div>';

if (!empty($data['error'])) {
	echo '<div class="gd-banner gd-err" style="white-space:pre-wrap"><b>Erro:</b> '.h($data['error']).'</div>';
}

echo '<form class="gd-card" method="get" action="zabbix.php">';
echo '<input type="hidden" name="action" value="godesk.flows.history">';
echo '<div class="gd-row">';
echo '<div class="gd-field"><label>Buscar <span class="gd-muted">(host, trigger, chamado, caminho...)</span></label><input type="text" name="q" value="'.h($f['q'] ?? '').'"></div>';
echo '<div class="gd-field gd-field-tight"><label>Fluxo</label><input type="text" name="flow" value="'.h($f['flow'] ?? '').'"></div>';
echo '<div class="gd-field gd-field-tight"><label>Rule</label><input type="text" name="rule" value="'.h($f['rule'] ?? '').'"></div>';
echo '<div class="gd-field gd-field-tight"><label>Event ID</label><input type="text" name="event" value="'.h($f['event'] ?? '').'"></div>';
echo '<div class="gd-field gd-field-tight" style="min-width:90px;"><label>Limite</label><input type="number" name="limit" min="10" max="1000" value="'.h($f['limit'] ?? 200).'"></div>';
echo '<div class="gd-field gd-field-tight" style="min-width:auto;"><label>&nbsp;</label><button class="gd-btn gd-btn-primary" type="submit">🔎 Filtrar</button></div>';
echo '<div class="gd-field gd-field-tight" style="min-width:auto;"><label>&nbsp;</label><a class="gd-btn" href="zabbix.php?action=godesk.flows.history">Limpar</a></div>';
echo '</div>';
echo '</form>';

echo '<div class="gd-card">';
if (!$entries) {
	echo '<div class="gd-muted">Nenhuma execução registrada'.(array_filter([$f['q'] ?? '', $f['flow'] ?? '', $f['rule'] ?? '', $f['event'] ?? '']) ? ' com esses filtros' : ' ainda — o histórico começa a partir desta versão').'.</div>';
}
else {
	echo '<div class="gd-muted" style="margin-bottom:8px;">'.count($entries).' execuções. Clique numa linha para ver o caminho completo.</div>';
	echo '<div class="gd-flow-hist">';
	foreach ($entries as $e) {
		$time = strtotime((string)($e['time'] ?? '')) ?: 0;
		$action = (string)($e['action'] ?? '');
		if (!empty($e['error'])) {
			$pill = '<span class="gd-pill gd-false">erro — rule fixa</span>';
		}
		elseif ($action === 'skip') {
			$pill = '<span class="gd-pill gd-false">não abriu</span>';
		}
		else {
			$pill = '<span class="gd-pill gd-true">abre/segue</span>';
		}
		$mode = (($e['mode'] ?? '') === 'shadow') ? ' <span class="gd-pill">shadow</span>' : '';
		$esc = '';
		foreach ((array)($e['notes'] ?? []) as $n) {
			if (strpos((string)$n, ': aplicado') !== false) {
				$esc = ' <span class="gd-pill gd-warn-pill">escalou</span>';
			}
		}

		echo '<details class="gd-flow-hist-item">';
		echo '<summary>';
		echo '<span class="gd-flow-hist-time">'.h($time ? date('d/m H:i:s', $time) : '-').'</span>';
		echo '<span class="gd-flow-hist-host"><b>'.h($e['host'] ?? '').'</b><br><span class="gd-muted">'.h($e['trigger'] ?? '').'</span></span>';
		echo '<span>'.h($e['rule'] ?? '').' → <b>'.h($e['flow'] ?? '').'</b>'.$mode.'</span>';
		echo '<span>'.$pill.$esc.(!empty($e['ticket']) ? '<br><span class="gd-muted">'.h($e['ticket']).'</span>' : '').'</span>';
		echo '</summary>';

		echo '<div class="gd-flow-hist-body">';
		echo '<div class="gd-muted">Evento '.h($e['event_id'] ?? '').' · '.((($e['kind'] ?? '') === 'recovery') ? 'normalização' : 'problema').'</div>';
		if (!empty($e['error'])) {
			echo '<div class="gd-banner gd-err">'.h($e['error']).'</div>';
		}
		if (!empty($e['trace'])) {
			echo '<div class="gd-small-title" style="margin-top:8px;">Caminho</div><ol class="gd-flow-trace">';
			foreach ((array)$e['trace'] as $s) {
				$lab = $port_label[$s['port'] ?? ''] ?? ($s['port'] ?? '');
				echo '<li><b>'.h(($s['label'] ?? '') !== '' ? $s['label'] : ($s['node'] ?? '')).'</b> <span class="gd-muted">('.h($s['type'] ?? '').')</span>'.
					($lab !== '' ? ' → <span class="gd-pill">'.h($lab).'</span>' : '').
					(!empty($s['detail']) ? '<br><span class="gd-muted">'.h($s['detail']).'</span>' : '').'</li>';
			}
			echo '</ol>';
		}
		if (!empty($e['set'])) {
			echo '<div class="gd-small-title" style="margin-top:8px;">Campos definidos'.((($e['mode'] ?? '') === 'shadow') ? ' (só no log — shadow)' : '').'</div><ul class="gd-flow-res">';
			foreach ((array)$e['set'] as $a) {
				echo '<li><b>'.h($field_label[$a['field']] ?? $a['field']).'</b>: '.h($a['value']).'</li>';
			}
			echo '</ul>';
		}
		if (!empty($e['updates'])) {
			echo '<div class="gd-small-title" style="margin-top:8px;">Atualizar chamado</div><ul class="gd-flow-res">';
			foreach ((array)$e['updates'] as $u) {
				$parts = [];
				foreach ((array)($u['fields'] ?? []) as $a) {
					$parts[] = ($field_label[$a['field']] ?? $a['field']).' = '.$a['value'];
				}
				if (!empty($u['note'])) {
					$parts[] = 'nota: '.$u['note'];
				}
				if (!empty($u['teams'])) {
					$parts[] = 'Teams';
				}
				echo '<li><b>'.h($u['node'] ?? '').'</b>: '.h(implode(' · ', $parts)).(!empty($u['repeat']) ? ' <span class="gd-muted">(repete a cada reenvio)</span>' : '').'</li>';
			}
			echo '</ul>';
		}
		if (!empty($e['ticket_notes'])) {
			$where = ['internal' => 'ação interna', 'public' => 'ação visível', 'description' => 'descrição'];
			echo '<div class="gd-small-title" style="margin-top:8px;">Texto no chamado</div><ul class="gd-flow-res">';
			foreach ((array)$e['ticket_notes'] as $n) {
				echo '<li><b>'.h($where[$n['target'] ?? ''] ?? ($n['target'] ?? '')).'</b>: <span style="white-space:pre-wrap">'.h($n['text'] ?? '').'</span></li>';
			}
			echo '</ul>';
		}
		if (!empty($e['zabbix'])) {
			echo '<div class="gd-small-title" style="margin-top:8px;">Ação no Zabbix</div><ul class="gd-flow-res">';
			foreach ((array)$e['zabbix'] as $z) {
				$parts = [];
				if (!empty($z['message'])) {
					$parts[] = 'comentário: '.$z['message'];
				}
				if (!empty($z['ack'])) {
					$parts[] = 'ack';
				}
				if (isset($z['severity']) && (int)$z['severity'] >= 0) {
					$parts[] = 'severidade '.(int)$z['severity'];
				}
				if (!empty($z['suppress_hours'])) {
					$parts[] = 'suprimir '.(int)$z['suppress_hours'].'h';
				}
				echo '<li><b>'.h($z['node'] ?? '').'</b>: '.h(implode(' · ', $parts)).'</li>';
			}
			echo '</ul>';
		}
		if (!empty($e['emails'])) {
			echo '<div class="gd-small-title" style="margin-top:8px;">E-mail</div><ul class="gd-flow-res">';
			foreach ((array)$e['emails'] as $m) {
				echo '<li><b>'.h($m['subject'] ?? '').'</b> → '.h($m['to'] ?? '').(!empty($m['cc']) ? ' <span class="gd-muted">(cc '.h($m['cc']).')</span>' : '').'</li>';
			}
			echo '</ul>';
		}
		if (!empty($e['vars'])) {
			echo '<div class="gd-small-title" style="margin-top:8px;">Variáveis</div><ul class="gd-flow-res">';
			foreach ((array)$e['vars'] as $k => $v) {
				echo '<li><code>{var.'.h($k).'}</code> = '.h($v).'</li>';
			}
			echo '</ul>';
		}
		if (!empty($e['http'])) {
			echo '<div class="gd-small-title" style="margin-top:8px;">HTTP</div><ul class="gd-flow-res">';
			foreach ((array)$e['http'] as $c) {
				echo '<li><b>'.h($c['method'] ?? '').'</b> '.h($c['url'] ?? '').'</li>';
			}
			echo '</ul>';
		}
		if (!empty($e['notes'])) {
			echo '<div class="gd-small-title" style="margin-top:8px;">O que o goDesk fez</div><ul class="gd-flow-res">';
			foreach ((array)$e['notes'] as $n) {
				echo '<li>'.h($n).'</li>';
			}
			echo '</ul>';
		}
		echo '</div>';
		echo '</details>';
	}
	echo '</div>';
}
echo '</div>';

echo '</div></div>';
