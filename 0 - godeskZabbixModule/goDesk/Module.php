<?php
/**
 * goDesk - módulo do Zabbix Frontend
 *
 * Responsável por registrar entradas no menu do Zabbix e expor as telas do módulo.
 */

namespace Modules\GoDesk;

use Zabbix\Core\CModule;
use APP;
use CMenu;
use CMenuItem;

class Module extends CModule {

	/**
	 * init() roda quando o módulo é carregado pelo frontend.
	 * Aqui adicionamos o submenu em Monitoring.
	 */
	public function init(): void {
		APP::Component()->get('menu.main')
			->findOrAdd(_('Monitoring'))
			->getSubmenu()
			->insertAfter(_('Alerts'),
				(new CMenuItem(_('goDesk')))->setSubMenu(
					new CMenu([
						(new CMenuItem(_('Config (visualizar)')))->setAction('godesk.config.view'),
						(new CMenuItem(_('Config (editar)')))->setAction('godesk.config.edit')
					])
				)
			);
	}
}
