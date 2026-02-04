<?php
namespace Modules\GoDesk\Actions;

use CController;
use CControllerResponseRedirect;

class GoDeskSave extends CController {

	protected function checkPermissions(): bool {
		return $this->getUserType() >= USER_TYPE_ZABBIX_ADMIN;
	}

	protected function doAction(): void {
		$yaml = $this->getInput('yaml', '');
		$path = '/etc/zabbix/godesk-config.yaml';

		if (trim($yaml) === '') {
			$this->redirectError('Configuração vazia.');
			return;
		}

		// grava direto (777 permitido)
		if (file_put_contents($path, $yaml) === false) {
			$this->redirectError('Falha ao gravar o arquivo.');
			return;
		}

		$response = new CControllerResponseRedirect('zabbix.php?action=godesk.rules&saved=1');
		$this->setResponse($response);
	}

	private function redirectError(string $msg): void {
		$response = new CControllerResponseRedirect(
			'zabbix.php?action=godesk.rules&error='.urlencode($msg)
		);
		$this->setResponse($response);
	}
}
