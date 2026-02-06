<?php

namespace Modules\GoDesk;

use Zabbix\Core\CModule;
use APP;
use CMenu;
use CMenuItem;

class Module extends CModule {
	public function init(): void {
		APP::Component()->get('menu.main')
			->findOrAdd(_('Monitoring'))
			->getSubmenu()
			->insertAfter(_('Discovery'),
				(new CMenuItem(_('goDesk')))->setSubMenu(
					new CMenu([
						(new CMenuItem(_('Hello World')))->setAction('godesk.hello')
					])
				)
			);
	}
}
