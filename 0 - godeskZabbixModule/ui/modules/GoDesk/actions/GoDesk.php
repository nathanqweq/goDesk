<?php
namespace Modules\GoDesk\Actions;

use CController;
use CControllerResponseData;

class GoDesk extends CController {

	public function init(): void {
		// CSRF permanece ativo (padrão)
	}

	protected function checkInput(): bool {
		$fields = [
			'saved' => 'int32',
			'error' => 'string'
		];

		return $this->validateInput($fields);
	}

	protected function checkPermissions() {
		return $this->getUserType() >= USER_TYPE_ZABBIX_ADMIN;
	}

	protected function doAction(): void {
		$config_path = '/etc/zabbix/godesk-config.yaml';

		$yaml = '';
		$error = (string) $this->getInput('error', '');

		if (is_readable($config_path)) {
			$yaml = (string) file_get_contents($config_path);
		}
		else if ($error === '') {
			$error = 'Arquivo não é legível pelo frontend: '.$config_path;
		}

		$data = [
			'config_path' => $config_path,
			'yaml' => $yaml,
			'saved' => (int) $this->getInput('saved', 0),
			'error' => $error
		];

		$this->setResponse(new CControllerResponseData($data));
	}
}
