<?php

$page = (new CHtmlPage())
	->setTitle($data['title']);

$page->show();

echo '<style>
.godesk-wrap{max-width:1200px}
.godesk-banner{padding:10px 12px;border-radius:6px;margin:10px 0}
.godesk-ok{background:#e9f7ef;border:1px solid #b7e2c7}
.godesk-err{background:#fdecea;border:1px solid #f5c2c7}
.godesk-meta{color:#666;margin:6px 0 14px 0}
.godesk-textarea{width:100%;height:520px;font-family:ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;font-size:13px;padding:10px;border:1px solid #ccc;border-radius:6px}
.godesk-actions{margin-top:10px;display:flex;gap:10px;align-items:center}
.godesk-btn{padding:8px 12px;border:1px solid #888;border-radius:6px;background:#f6f6f6;cursor:pointer}
.godesk-btn:hover{background:#eee}
</style>';

echo '<div class="godesk-wrap">';
echo '<h1>⚙️ Editor do goDesk</h1>';
echo '<div class="godesk-meta"><b>Arquivo:</b> '.htmlspecialchars($data['path']).'</div>';

if (!empty($data['status'])) {
	echo '<div class="godesk-banner godesk-ok">'.htmlspecialchars($data['status']).'</div>';
	if (!empty($data['backup'])) {
		echo '<div class="godesk-meta"><b>Backup:</b> '.htmlspecialchars($data['backup']).'</div>';
	}
}

if (!empty($data['error'])) {
	echo '<div class="godesk-banner godesk-err"><b>Erro:</b> '.htmlspecialchars($data['error']).'</div>';
}

echo '<form method="post" action="zabbix.php?action=godesk.config.edit">';
echo '<input type="hidden" name="save" value="1">';

echo '<textarea class="godesk-textarea" name="yaml_text">'.htmlspecialchars($data['yaml_text']).'</textarea>';

echo '<div class="godesk-actions">';
echo '<button class="godesk-btn" type="submit">💾 Salvar</button>';
echo '<a class="godesk-btn" href="zabbix.php?action=godesk.config.edit" style="text-decoration:none;color:inherit">↩ Recarregar</a>';
echo '</div>';

echo '</form>';
echo '</div>';
