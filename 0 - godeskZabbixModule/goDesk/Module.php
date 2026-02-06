<?php
/**
 * goDesk - Zabbix Frontend Module
 *
 * Responsável por inicializar o módulo no frontend e registrar itens de menu.
 */

namespace Modules\GoDesk;

use Zabbix\Core\CModule;
use APP;
use CMenu;
use CMenuItem;

class Module extends CModule {

	/**
	 * Executado quando o módulo é carregado pelo frontend.
	 * Adiciona o submenu "goDesk" em Monitoring.
	 */
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
