<?php
/**
 * Action: godesk.config.validate
 *
 * Confere se operator/oper_group existem no TopDesk (chamado pelo botão
 * "🔍 Testar" no card de Cliente). Como as credenciais do TopDesk só
 * existem no godesk-service.env do servidor, isso passa pelo
 * godesk-client (mesmo binário já usado pra sincronizar a config).
 */

namespace Modules\GoDesk\Actions;

use CController;
use CControllerResponseData;

class ConfigValidate extends CController {

	public function init(): void {
		$this->disableCsrfValidation();
	}

	protected function checkInput(): bool {
		$fields = [
			'operator' => 'string',
			'oper_group' => 'string'
		];

		return $this->validateInput($fields);
	}

	protected function checkPermissions(): bool {
		return (isset(\CWebUser::$data['type']) && \CWebUser::$data['type'] == USER_TYPE_SUPER_ADMIN);
	}

	protected function doAction(): void {
		$bin = '/usr/bin/godesk-client';

		if (!is_file($bin) || !is_executable($bin)) {
			$this->setResponse(new CControllerResponseData([
				'result' => ['error' => 'godesk-client não instalado neste host — instale o pacote godesk-client pra usar o teste.']
			]));
			return;
		}

		$payload = json_encode([
			'operator' => (string)$this->getInput('operator', ''),
			'oper_group' => (string)$this->getInput('oper_group', '')
		]);

		$output = [];
		$exit_code = 0;
		exec(escapeshellarg($bin).' validate '.escapeshellarg($payload).' 2>&1', $output, $exit_code);

		if ($exit_code !== 0) {
			$this->setResponse(new CControllerResponseData([
				'result' => ['error' => trim(implode("\n", $output))]
			]));
			return;
		}

		$decoded = json_decode(implode("\n", $output), true);
		if (!is_array($decoded)) {
			$this->setResponse(new CControllerResponseData([
				'result' => ['error' => 'resposta inesperada do godesk-client: '.implode("\n", $output)]
			]));
			return;
		}

		$this->setResponse(new CControllerResponseData(['result' => $decoded]));
	}
}
