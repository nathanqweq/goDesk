<?php
namespace Modules\GoDesk;

use Zabbix\Core\CModule;
use APP;
use CMenuItem;

class Module extends CModule {
	public function init(): void {
		APP::Component()->get('menu.main')
			->findOrAdd(_('Administration'))
			->getSubmenu()
			->add((new CMenuItem(_('GoDesk')))->setAction('godesk.rules'));
	}
}
