<?php
/**
 * Action: godesk.topology.view
 *
 * Visualização SOMENTE LEITURA da dependência v2 (arquitetura por
 * localidade), pra equipe de atendimento: desenho, simulação (marcar hosts
 * DOWN / evento de status) e estado atual do Zabbix. Não grava nada — a
 * edição continua em godesk.topology.edit (só Super admin).
 *
 * Acesso: qualquer usuário logado cujo role tenha o módulo goDesk liberado
 * (Users → User roles → Access to modules). O estado atual respeita as
 * permissões do usuário: host que ele não enxerga aparece como UP.
 */

namespace Modules\GoDesk\Actions;

use CControllerResponseData;

class TopologyView extends DependenciesV2Edit {

	protected function checkInput(): bool {
		return true;
	}

	protected function checkPermissions(): bool {
		return ($this->getUserType() >= USER_TYPE_ZABBIX_USER);
	}

	protected function doAction(): void {
		$file = $this->loadFile();

		$this->setResponse(new CControllerResponseData([
			'title' => _('goDesk - Arquitetura (visualizar)'),
			'error' => $file['error'],
			'sites' => $file['sites'],
			'can_edit' => ($this->getUserType() == USER_TYPE_SUPER_ADMIN),
			'defaults' => ['root' => 'ZABBIX', 'down_event' => '*Unavailable by ICMP ping*']
		]));
	}
}
