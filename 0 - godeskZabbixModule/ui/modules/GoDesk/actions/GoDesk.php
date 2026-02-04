<?php
namespace Modules\GoDesk\Actions;

use CController;
use CControllerResponseData;

class GoDesk extends CController {

	protected function checkPermissions(): bool {
		return $this->getUserType() >= USER_TYPE_ZABBIX_ADMIN;
	}

	protected function doAction(): void {
		$config_path = '/etc/zabbix/godesk-config.yaml';

		$data = [
			'config_path' => $config_path,
			'yaml' => is_readable($config_path) ? file_get_contents($config_path) : '',
			'saved' => (int) $this->getInput('saved', 0),
			'error' => (string) $this->getInput('error', '')
		];

		$this->setResponse(new CControllerResponseData($data));
	}
}
