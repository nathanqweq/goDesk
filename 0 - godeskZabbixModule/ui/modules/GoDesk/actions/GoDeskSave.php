<?php
namespace Modules\GoDesk\Actions;

use CController;
use CControllerResponseRedirect;

class GoDeskSave extends CController {

	public function init(): void {
		// CSRF ativo
	}

	protected function checkInput(): bool {
		$fields = [
			'yaml' => 'required|string'
		];

		return $this->validateInput($fields);
	}

	protected function checkPermissions(): bool {
		$permit_user_types = [USER_TYPE_ZABBIX_ADMIN, USER_TYPE_SUPER_ADMIN];
		return in_array($this->getUserType(), $permit_user_types);
	}

	protected function doAction(): void {
		$yaml = (string) $this->getInput('yaml', '');
		$path = '/etc/zabbix/godesk-config.yaml';

		if (trim($yaml) === '') {
			$this->setResponse(new CControllerResponseRedirect(
				'zabbix.php?action=godesk.rules&error='.urlencode('Configuração vazia.')
			));
			return;
		}

		$ok = @file_put_contents($path, $yaml);

		if ($ok === false) {
			$this->setResponse(new CControllerResponseRedirect(
				'zabbix.php?action=godesk.rules&error='.urlencode('Falha ao gravar o arquivo: '.$path)
			));
			return;
		}

		$this->setResponse(new CControllerResponseRedirect(
			'zabbix.php?action=godesk.rules&saved=1'
		));
	}
}
