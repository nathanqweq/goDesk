<?php
/**
 * Action: godesk.config.view
 *
 * Lê o arquivo YAML de configuração do goDesk e envia os dados para a view
 * que exibe a configuração de forma amigável.
 */

namespace Modules\GoDesk\Actions;

use CController;
use CControllerResponseData;

class ConfigView extends CController {

	/** Caminho do arquivo de configuração (ajuste aqui se mudar) */
	private string $config_path = '/etc/zabbix/godesk/godesk-config.yaml';

	public function init(): void {
		// Tela somente leitura (GET), então mantemos simples.
		$this->disableCsrfValidation();
	}

	protected function checkInput(): bool {
		// Sem parâmetros esperados no GET por enquanto.
		return true;
	}

	protected function checkPermissions(): bool {
		// Pode abrir para todos, ou restringir; por enquanto liberado.
		// Se quiser travar: USER_TYPE_ZABBIX_ADMIN / SUPER_ADMIN etc.
		return true;
	}

	/**
	 * Carrega e faz parse do YAML para array PHP.
	 * Retorna um array com dados e/ou mensagem de erro.
	 */
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
