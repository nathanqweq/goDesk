<?php
/**
 * goDesk - Zabbix Frontend Module
 *
 * Registra o menu e inicializa o módulo.
 */

namespace Modules\GoDesk;

use Zabbix\Core\CModule;
use APP;
use CMenu;
use CMenuItem;

class Module extends CModule {
	public function init(): void {
		// Uma entrada por assunto: o Super admin abre a tela de edição e os
		// demais a de visualização (as actions de edição já recusam quem não
		// é admin). Na página, as abas (views/inc.nav.php) seguem a mesma regra.
		$admin = (\CWebUser::getType() == USER_TYPE_SUPER_ADMIN);

		$items = [
			(new CMenuItem(_('Rules')))
				->setAction($admin ? 'godesk.config.edit' : 'godesk.config.view')
				->setAliases($admin ? ['godesk.config.view', 'godesk.config.validate'] : []),
			(new CMenuItem(_('Fluxos')))
				->setAction($admin ? 'godesk.flows.edit' : 'godesk.flows.view')
				->setAliases($admin ? ['godesk.flows.view'] : [])
		];
		if ($admin) {
			$items[] = (new CMenuItem(_('Histórico de fluxos')))->setAction('godesk.flows.history');
		}
		$items[] = (new CMenuItem(_('Arquitetura')))
			->setAction($admin ? 'godesk.topology.edit' : 'godesk.topology.view')
			->setAliases($admin ? ['godesk.topology.view'] : []);
		if ($admin) {
			$items[] = (new CMenuItem(_('Dependências v1')))
				->setAction('godesk.dependencies.edit')
				->setAliases(['godesk.dependencies.triggers']);
		}

		$menu = (new CMenuItem('goDesk'))->setSubMenu(new CMenu($items));
		if (method_exists($menu, 'setId')) {
			$menu->setId('godesk');
		}
		$icon = self::icon();
		if ($icon !== null) {
			$menu->setIcon($icon);
		}

		// seção própria na barra lateral, logo depois de Reports (que existe
		// pra todos os tipos de usuário — Alerts só existe pro admin)
		$main = APP::Component()->get('menu.main');
		if (method_exists($main, 'find') && $main->find(_('Reports')) !== null) {
			$main->insertAfter(_('Reports'), $menu);
		}
		else {
			$main->add($menu);
		}
	}

	// ícone do menu: as constantes mudam entre versões do Zabbix, então usa a
	// primeira que existir (sem nenhuma, o item fica sem ícone)
	private static function icon(): ?string {
		foreach (['ZBX_ICON_ENVELOPE_FILLED', 'ZBX_ICON_ALERTS', 'ZBX_ICON_BELL', 'ZBX_ICON_REPORTS'] as $c) {
			if (defined($c)) {
				return constant($c);
			}
		}
		return null;
	}
}
