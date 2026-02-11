<?php
/**
 * Action: godesk.config.view
 *
 * Exibe a configuração YAML do goDesk de forma amigável.
 */

namespace Modules\GoDesk\Actions;

use CController;
use CControllerResponseData;

class ConfigView extends CController {

	private string $config_path = '/etc/zabbix/godesk/godesk-config.yaml';

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
			return ['_error' => 'Arquivo não encontrado: '.$this->config_path];
		}

		if (!is_readable($this->config_path)) {
			return ['_error' => 'Sem permissão de leitura: '.$this->config_path];
		}

		if (!function_exists('yaml_parse_file')) {
			return ['_error' => 'Extensão PHP yaml não instalada (yaml_parse_file).'];
		}

		$parsed = @yaml_parse_file($this->config_path);

		if ($parsed === false || !is_array($parsed)) {
			return ['_error' => 'YAML inválido ou vazio.'];
		}

		$parsed['default'] ??= [];
		$parsed['clients'] ??= [];

		$parsed['default']['client'] ??= '';
		$parsed['default']['priority'] ??= '';
		$parsed['default']['topdesk'] ??= [];

		foreach ($parsed['clients'] as $rule => $c) {
			if (!is_array($c)) {
				$parsed['clients'][$rule] = [];
			}
			$parsed['clients'][$rule]['client'] ??= '';
			$parsed['clients'][$rule]['priority'] ??= '';
			$parsed['clients'][$rule]['topdesk'] ??= [];
		}

		return $parsed;
	}

	protected function doAction(): void {
		$config = $this->loadConfig();

		$this->setResponse(new CControllerResponseData([
			'title' => _('goDesk - Configuração'),
			'path' => $this->config_path,
			'config' => $config
		]));
	}
}
