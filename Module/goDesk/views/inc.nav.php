<?php
/**
 * Abas do goDesk no topo de cada página (incluído pelas views).
 *
 * Uma aba por assunto: o Super admin cai na tela de edição e os demais na
 * de visualização (mesma regra do menu em Module.php). Nas telas que têm as
 * duas versões, o admin ganha o atalho "Visualizar" / "Editar".
 */
$gdNavAdmin = (CWebUser::getType() == USER_TYPE_SUPER_ADMIN);
$gdNavAction = (string) ($_REQUEST['action'] ?? '');

// [rótulo, action do admin, action dos demais (null = só admin), actions que marcam a aba]
$gdNavTabs = [
	['📋 Rules', 'godesk.config.edit', 'godesk.config.view', ['godesk.config.edit', 'godesk.config.view', 'godesk.config.validate']],
	['🔀 Fluxos', 'godesk.flows.edit', 'godesk.flows.view', ['godesk.flows.edit', 'godesk.flows.view']],
	['🕘 Histórico de fluxos', 'godesk.flows.history', null, ['godesk.flows.history']],
	['🗺 Arquitetura', 'godesk.topology.edit', 'godesk.topology.view', ['godesk.topology.edit', 'godesk.topology.view']],
	['🔗 Dependências v1', 'godesk.dependencies.edit', null, ['godesk.dependencies.edit', 'godesk.dependencies.triggers']]
];

// editar ↔ visualizar (só admin)
$gdNavSwitch = [
	'godesk.config.edit' => ['godesk.config.view', '👁 Visualizar'],
	'godesk.config.view' => ['godesk.config.edit', '✎ Editar'],
	'godesk.flows.edit' => ['godesk.flows.view', '👁 Visualizar'],
	'godesk.flows.view' => ['godesk.flows.edit', '✎ Editar'],
	'godesk.topology.edit' => ['godesk.topology.view', '👁 Visualizar'],
	'godesk.topology.view' => ['godesk.topology.edit', '✎ Editar']
];

$gdNavHtml = '<nav class="gd-nav"><div class="gd-nav-tabs">';
foreach ($gdNavTabs as [$gdLabel, $gdAdminAction, $gdUserAction, $gdMatch]) {
	$gdTarget = $gdNavAdmin ? $gdAdminAction : $gdUserAction;
	if ($gdTarget === null) {
		continue;
	}
	$gdActive = in_array($gdNavAction, $gdMatch, true);
	$gdNavHtml .= '<a class="gd-nav-tab'.($gdActive ? ' is-active' : '').'" href="zabbix.php?action='
		.htmlspecialchars($gdTarget, ENT_QUOTES, 'UTF-8').'"'.($gdActive ? ' aria-current="page"' : '').'>'
		.htmlspecialchars($gdLabel, ENT_QUOTES, 'UTF-8').'</a>';
}
$gdNavHtml .= '</div>';
if ($gdNavAdmin && isset($gdNavSwitch[$gdNavAction])) {
	[$gdTo, $gdLabel] = $gdNavSwitch[$gdNavAction];
	$gdNavHtml .= '<a class="gd-nav-switch" href="zabbix.php?action='.htmlspecialchars($gdTo, ENT_QUOTES, 'UTF-8').'">'
		.htmlspecialchars($gdLabel, ENT_QUOTES, 'UTF-8').'</a>';
}
$gdNavHtml .= '</nav>';
echo $gdNavHtml;
