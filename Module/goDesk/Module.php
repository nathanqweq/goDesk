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
		APP::Component()->get('menu.main')
			->findOrAdd(_('Alerts'))
			->getSubmenu()
			->insertAfter(_('Scripts'),
				(new CMenuItem(_('goDesk')))->setSubMenu(
					new CMenu([
						(new CMenuItem(_('Config (visualizar)')))->setAction('godesk.config.view'),
						(new CMenuItem(_('Config (editar)')))->setAction('godesk.config.edit')
					])
				)
			);
	}
}
