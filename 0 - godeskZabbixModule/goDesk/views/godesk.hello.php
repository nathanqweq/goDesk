<?php

(new CHtmlPage())
	->setTitle($data['title'])
	->show();

$config = $data['config'];

echo '<style>
.godesk-card {
	background: #fff;
	border: 1px solid #dcdcdc;
	border-radius: 6px;
	padding: 15px;
	margin-bottom: 15px;
	box-shadow: 0 1px 3px rgba(0,0,0,0.08);
}

.godesk-title {
	font-size: 18px;
	font-weight: bold;
	margin-bottom: 10px;
}

.godesk-tag {
	display: inline-block;
	background: #eef3ff;
	border: 1px solid #ccd7ff;
	padding: 4px 8px;
	margin: 3px;
	border-radius: 4px;
	font-size: 12px;
}

.godesk-section {
	margin-top: 30px;
}

.godesk-bool-true {
	color: green;
	font-weight: bold;
}

.godesk-bool-false {
	color: red;
	font-weight: bold;
}
</style>';

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
