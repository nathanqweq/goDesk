<?php

$page = (new CHtmlPage())
	->setTitle(_('GoDesk - Config'));

if (!empty($data['error'])) {
	$page->addItem(makeMessageBox(ZBX_STYLE_MSG_BAD, $data['error'], null, true));
}
elseif (!empty($data['saved'])) {
	$page->addItem(makeMessageBox(ZBX_STYLE_MSG_GOOD, _('Configuração salva com sucesso.'), null, true));
}

$form = (new CForm('post', 'zabbix.php'))
	->addVar('action', 'godesk.rules.save');

$form->addItem(new CLabel(_('Arquivo: ').$data['config_path']));

$form->addItem(
	(new CTextArea('yaml', $data['yaml']))
		->setWidth(ZBX_TEXTAREA_BIG_WIDTH)
		->setRows(28)
);

$form->addItem(new CSubmit('save', _('Salvar')));

$page->addItem($form);
$page->show();
