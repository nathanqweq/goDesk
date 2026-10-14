<?php
/**
 * Action: godesk.dependencies.triggers
 *
 * Lista os nomes de trigger (já com macros expandidas) de um host, pra
 * sugerir no campo "Evento a observar" da tela de Dependências.
 */

namespace Modules\GoDesk\Actions;

use CController;
use CControllerResponseData;

class DependenciesTriggers extends CController {

	public function init(): void {
		$this->disableCsrfValidation();
	}

	protected function checkInput(): bool {
		return $this->validateInput(['host' => 'string']);
	}

	protected function checkPermissions(): bool {
		return (isset(\CWebUser::$data['type']) && \CWebUser::$data['type'] == USER_TYPE_SUPER_ADMIN);
	}

	protected function doAction(): void {
		$host = trim((string)$this->getInput('host', ''));
		if ($host === '') {
			$this->setResponse(new CControllerResponseData(['result' => ['triggers' => []]]));
			return;
		}

		$hosts = \API::Host()->get([
			'output' => ['hostid'],
			'filter' => ['host' => $host]
		]);
		if (!$hosts) {
			$this->setResponse(new CControllerResponseData([
				'result' => ['error' => 'host não encontrado: '.$host]
			]));
			return;
		}

		$triggers = \API::Trigger()->get([
			'output' => ['description'],
			'hostids' => [$hosts[0]['hostid']],
			'expandDescription' => true
		]);

		$names = array_values(array_unique(array_column($triggers, 'description')));
		sort($names, SORT_NATURAL | SORT_FLAG_CASE);

		$this->setResponse(new CControllerResponseData(['result' => ['triggers' => $names]]));
	}
}
