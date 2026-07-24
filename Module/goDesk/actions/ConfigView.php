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

	private function fillTopdeskDefaults(array &$td): void {
		$td['send_more_info'] ??= false;
		$td['more_info_text'] ??= '';
		$td['adicional_cresol'] ??= false;
		$td['send_email'] ??= false;
		$td['email_to'] ??= '';
		$td['email_cc'] ??= '';
		$td['once_per_day'] ??= false;
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
		$parsed['default']['client'] ??= '';
		$parsed['default']['priority'] ??= '';
		$parsed['default']['topdesk'] ??= [];
		$this->fillTopdeskDefaults($parsed['default']['topdesk']);

		$is_new_format = !empty($parsed['rules']) && is_array($parsed['rules']);

		if ($is_new_format) {
			$named_clients = is_array($parsed['clients'] ?? null) ? $parsed['clients'] : [];
			foreach ($named_clients as $name => $c) {
				if (!is_array($c)) {
					$named_clients[$name] = [];
				}
				$named_clients[$name]['topdesk'] ??= [];
				$this->fillTopdeskDefaults($named_clients[$name]['topdesk']);
			}

			$rules = $parsed['rules'];
		}
		else {
			$named_clients = [];
			$rules = is_array($parsed['clients'] ?? null) ? $parsed['clients'] : [];
		}

		foreach ($rules as $rule => $r) {
			if (!is_array($r)) {
				$rules[$rule] = [];
			}
			$rules[$rule]['client'] ??= '';
			$rules[$rule]['priority'] ??= '';
			$rules[$rule]['custom_status'] ??= false;
			$rules[$rule]['status_open'] ??= '';
			$rules[$rule]['status_update'] ??= '';
			$rules[$rule]['topdesk'] ??= [];
			$this->fillTopdeskDefaults($rules[$rule]['topdesk']);
		}

		return ['default' => $parsed['default'], 'rules' => $rules, 'named_clients' => $named_clients];
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
