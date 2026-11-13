<?php
/**
 * goDesk - Zabbix Frontend Module
 *
 * Registra itens de menu e inicializa o módulo.
 */

namespace Modules\GoDesk;

use Zabbix\Core\CModule;
use APP;
use CMenu;
use CMenuItem;

class Module extends CModule {
	public function init(): void {
		// telas de edição só aparecem pro Super admin (as actions já
		// recusam os demais); o atendimento vê só as de visualização
		$admin = (\CWebUser::getType() == USER_TYPE_SUPER_ADMIN);

		$items = [
			(new CMenuItem(_('Config (visualizar)')))->setAction('godesk.config.view'),
			(new CMenuItem(_('Arquitetura (visualizar)')))->setAction('godesk.topology.view')
		];
		if ($admin) {
			$items[] = (new CMenuItem(_('Config (editar)')))->setAction('godesk.config.edit');
			$items[] = (new CMenuItem(_('Dependências')))->setAction('godesk.dependencies.edit');
			$items[] = (new CMenuItem(_('Dependências v2 (arquitetura)')))->setAction('godesk.topology.edit');
		}

		APP::Component()->get('menu.main')
			->findOrAdd(_('Alerts'))
			->getSubmenu()
			->insertAfter(_('Scripts'),
				(new CMenuItem(_('goDesk')))->setSubMenu(new CMenu($items))
			);
	}
}
