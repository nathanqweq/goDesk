<?php
(new CHtmlPage())
	->setTitle($data['title'])
	->show();

// $data['config'] vem da action
// $data['path'] é o caminho do arquivo


$config = $data['config'];

echo '<h1>⚙️ goDesk Config Viewer</h1>';

//
// DEFAULT
//
if (isset($config['default'])) {
	$d = $config['default'];

	echo '<div class="godesk-section">';
	echo '<div class="godesk-title">📦 Default Config</div>';
	echo '<div class="godesk-card">';

	echo '<b>Urgency:</b> '.$d['urgency'].'<br>';
	echo '<b>Impact:</b> '.$d['impact'].'<br>';

	$auto = $d['autoclose'] ? '<span class="godesk-bool-true">true</span>' : '<span class="godesk-bool-false">false</span>';
	echo '<b>Autoclose:</b> '.$auto.'<br><br>';

	echo '<b>Tags:</b><br>';
	foreach ($d['tags'] as $k => $v) {
		echo '<span class="godesk-tag">'.$k.': '.$v.'</span>';
	}

	echo '</div>';
	echo '</div>';
}

//
// CLIENTES
//
if (isset($config['clients'])) {
	echo '<div class="godesk-section">';
	echo '<div class="godesk-title">👥 Clients</div>';

	foreach ($config['clients'] as $client => $c) {

		echo '<div class="godesk-card">';
		echo '<div class="godesk-title">🏢 '.$client.'</div>';

		if (isset($c['urgency']))
			echo '<b>Urgency:</b> '.$c['urgency'].'<br>';

		if (isset($c['impact']))
			echo '<b>Impact:</b> '.$c['impact'].'<br>';

		if (isset($c['autoclose'])) {
			$auto = $c['autoclose'] ? '<span class="godesk-bool-true">true</span>' : '<span class="godesk-bool-false">false</span>';
			echo '<b>Autoclose:</b> '.$auto.'<br>';
		}

		if (isset($c['tags'])) {
			echo '<br><b>Tags:</b><br>';
			foreach ($c['tags'] as $k => $v) {
				echo '<span class="godesk-tag">'.$k.': '.$v.'</span>';
			}
		}

		echo '</div>';
	}

	echo '</div>';
}
