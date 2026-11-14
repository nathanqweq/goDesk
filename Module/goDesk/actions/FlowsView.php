<?php
/**
 * Action: godesk.flows.view
 *
 * Visualização SOMENTE LEITURA dos fluxos, pra equipe de atendimento:
 * desenho, calendários e o 🧪 Testar com problemas reais. Não grava nada —
 * a edição continua em godesk.flows.edit (só Super admin).
 *
 * Acesso: qualquer usuário logado cujo role tenha o módulo goDesk liberado.
 * O Testar usa as permissões de host do próprio usuário.
 */

namespace Modules\GoDesk\Actions;

use CControllerResponseData;

class FlowsView extends FlowsEdit {

	protected function checkInput(): bool {
		return true;
	}

	protected function checkPermissions(): bool {
		return ($this->getUserType() >= USER_TYPE_ZABBIX_USER);
	}

	protected function doAction(): void {
		$file = $this->loadFile();

		$this->setResponse(new CControllerResponseData([
			'title' => _('goDesk - Fluxos (visualizar)'),
			'error' => $file['error'],
			'data' => $file['data'],
			'can_edit' => ($this->getUserType() == USER_TYPE_SUPER_ADMIN)
		]));
	}
}
