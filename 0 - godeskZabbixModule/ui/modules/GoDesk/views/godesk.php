<?php
$page = (new CHtmlPage())
	->setTitle(_('GoDesk - Regras de Negócio'));

if ($data['error'] !== '') {
	$page->addItem((new CMessageHelper())->setError($data['error']));
}
elseif ($data['saved']) {
	$page->addItem((new CMessageHelper())->setSuccess(_('Configuração salva com sucesso.')));
}

$form = (new CForm('post', 'zabbix.php?action=godesk.rules.save'))
	->addVar('action', 'godesk.rules.save');

$form->addItem(new CLabel(_('Arquivo: ').$data['config_path']));

$form->addItem(
	(new CTextArea('yaml', $data['yaml']))
		->setWidth(ZBX_TEXTAREA_BIG_WIDTH)
		->setRows(30)
);

$form->addItem(new CSubmit('save', _('Salvar')));

$page->addItem($form);
$page->show();
