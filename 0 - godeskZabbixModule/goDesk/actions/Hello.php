<?php

namespace Modules\GoDesk\Actions;

use CController;
use CControllerResponseData;

class Hello extends CController {

	private $config_path = '/etc/zabbix/godesk-config.yaml';

	public function init(): void {
		$this->disableCsrfValidation();
	}

	protected function checkInput(): bool {
		return true;
	}

	protected function checkPermissions(): bool {
		return true;
	}

	private function loadConfig(): array {
		if (!file_exists($this->config_path)) {
			return [
				'error' => 'Arquivo de config não encontrado',
				'path' => $this->config_path
			];
		}

		if (!is_readable($this->config_path)) {
			return [
				'error' => 'Sem permissão de leitura',
				'path' => $this->config_path
			];
		}

		if (!function_exists('yaml_parse_file')) {
			return [
				'error' => 'Extensão PHP yaml não instalada'
			];
		}

		$data = yaml_parse_file($this->config_path);

		if ($data === false) {
			return [
				'error' => 'Erro ao fazer parse do YAML'
			];
		}

		return $data;
	}

	protected function doAction(): void {

		$config = $this->loadConfig();

		$data = [
			'title' => _('goDesk config viewer'),
			'config' => $config
		];

		$this->setResponse(new CControllerResponseData($data));
	}
}
