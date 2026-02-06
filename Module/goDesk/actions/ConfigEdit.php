<?php
/**
 * Action: godesk.config.edit
 *
 * Form de edição da configuração do goDesk.
 * - Lê YAML do disco
 * - Renderiza campos (default + clients)
 * - No POST: normaliza + valida + gera YAML + salva (backup + escrita atômica)
 */

namespace Modules\GoDesk\Actions;

use CController;
use CControllerResponseData;

class ConfigEdit extends CController {

	private string $config_path = '/etc/zabbix/godesk/godesk-config.yaml';

	public function init(): void {
		// Simplificado por enquanto (sem CSRF). Podemos reforçar depois.
		$this->disableCsrfValidation();
	}

	protected function checkInput(): bool {
		$fields = [
			'save' => 'in 0,1',
			'default' => 'array',
			'clients' => 'array'
		];

		return $this->validateInput($fields);
	}

	protected function checkPermissions(): bool {
		// Edição: recomendável restringir a Super Admin.
		return (isset(\CWebUser::$data['type']) && \CWebUser::$data['type'] == USER_TYPE_SUPER_ADMIN);
	}

	private function loadConfig(): array {
		if (!file_exists($this->config_path)) {
			return ['default' => [], 'clients' => []];
		}

		if (!is_readable($this->config_path)) {
			return ['default' => [], 'clients' => [], '_error' => 'Sem permissão de leitura: '.$this->config_path];
		}

		if (!function_exists('yaml_parse_file')) {
			return ['default' => [], 'clients' => [], '_error' => 'Extensão PHP yaml não instalada (yaml_parse_file).'];
		}

		$parsed = @yaml_parse_file($this->config_path);

		if ($parsed === false || !is_array($parsed)) {
			return ['default' => [], 'clients' => [], '_error' => 'YAML inválido ou vazio.'];
		}

		$parsed['default'] ??= [];
		$parsed['clients'] ??= [];

		// Garantir estrutura nova (topdesk)
		$parsed['default']['topdesk'] ??= [];
		foreach ($parsed['clients'] as $k => $v) {
			if (!is_array($v)) {
				$parsed['clients'][$k] = [];
			}
			$parsed['clients'][$k]['topdesk'] ??= [];
		}

		return $parsed;
	}

	private function toBool($v): bool {
		return ($v === 1 || $v === '1' || $v === true || $v === 'true' || $v === 'on');
	}

	/**
	 * Normaliza o payload do formulário para o schema final do YAML.
	 */
	private function normalizeConfigFromPost(array $post_default, array $post_clients): array {
		$def_td = (array)($post_default['topdesk'] ?? []);

		$config = [
			'default' => [
				'urgency' => (string)($post_default['urgency'] ?? ''),
				'impact' => (string)($post_default['impact'] ?? ''),
				'autoclose' => $this->toBool($post_default['autoclose'] ?? false),
				'topdesk' => [
					'contract' => (string)($def_td['contract'] ?? ''),
					'operator' => (string)($def_td['operator'] ?? ''),
					'oper_group' => (string)($def_td['oper_group'] ?? ''),
					'main_caller' => (string)($def_td['main_caller'] ?? ''),
					'secundary_caller' => (string)($def_td['secundary_caller'] ?? ''),
					'sla' => (string)($def_td['sla'] ?? ''),
					'category' => (string)($def_td['category'] ?? ''),
					'sub_category' => (string)($def_td['sub_category'] ?? ''),
					'call_type' => (string)($def_td['call_type'] ?? '')
				]
			],
			'clients' => []
		];

		// clients é uma lista: clients[0][name], clients[0][topdesk][...]
		foreach ($post_clients as $row) {
			$name = trim((string)($row['name'] ?? ''));
			if ($name === '') {
				continue;
			}

			$td = (array)($row['topdesk'] ?? []);

			$config['clients'][$name] = [
				'autoclose' => $this->toBool($row['autoclose'] ?? false),
				'urgency' => (string)($row['urgency'] ?? ''),
				'impact' => (string)($row['impact'] ?? ''),
				'topdesk' => [
					'contract' => (string)($td['contract'] ?? ''),
					'operator' => (string)($td['operator'] ?? ''),
					'oper_group' => (string)($td['oper_group'] ?? ''),
					'main_caller' => (string)($td['main_caller'] ?? ''),
					'secundary_caller' => (string)($td['secundary_caller'] ?? ''),
					'sla' => (string)($td['sla'] ?? ''),
					'category' => (string)($td['category'] ?? ''),
					'sub_category' => (string)($td['sub_category'] ?? ''),
					'call_type' => (string)($td['call_type'] ?? '')
				]
			];
		}

		return $config;
	}

	private function validateConfig(array $cfg): array {
		if (!isset($cfg['default']) || !isset($cfg['clients'])) {
			return ['ok' => false, 'error' => 'Config inválida: faltam as chaves default/clients.'];
		}

		if (!isset($cfg['default']['topdesk']) || !is_array($cfg['default']['topdesk'])) {
			return ['ok' => false, 'error' => 'Config inválida: default.topdesk ausente.'];
		}

		// Checagem leve (deixa campos opcionais, mas garante chaves esperadas)
		foreach (['contract','operator','oper_group','main_caller','secundary_caller','sla','category','sub_category','call_type'] as $k) {
			if (!array_key_exists($k, $cfg['default']['topdesk'])) {
				return ['ok' => false, 'error' => 'Config inválida: default.topdesk.'.$k.' ausente.'];
			}
		}

		return ['ok' => true, 'error' => null];
	}

	/**
	 * Salva o YAML com backup + escrita atômica no mesmo diretório do arquivo.
	 */
	private function saveYamlAtomic(string $yaml_text): array {
		$dir = dirname($this->config_path);

		if (file_exists($this->config_path)) {
			if (!is_writable($this->config_path)) {
				return ['ok' => false, 'error' => 'Sem permissão de escrita: '.$this->config_path];
			}
		}
		else {
			if (!is_writable($dir)) {
				return ['ok' => false, 'error' => 'Sem permissão para criar arquivo em: '.$dir];
			}
		}

		$tmp = $dir.'/'.basename($this->config_path).'.tmp';
		$bak = $dir.'/'.basename($this->config_path).'.bak.'.date('Ymd-His');

		if (file_exists($this->config_path) && !@copy($this->config_path, $bak)) {
			return ['ok' => false, 'error' => 'Falha ao criar backup em: '.$bak];
		}

		$bytes = @file_put_contents($tmp, $yaml_text, LOCK_EX);
		if ($bytes === false) {
			return ['ok' => false, 'error' => 'Falha ao escrever tmp: '.$tmp];
		}

		@chmod($tmp, 0640);

		if (!@rename($tmp, $this->config_path)) {
			@unlink($tmp);
			return ['ok' => false, 'error' => 'Falha ao aplicar arquivo (rename).'];
		}

		return ['ok' => true, 'error' => null, 'backup' => (file_exists($bak) ? $bak : null)];
	}

	protected function doAction(): void {
		$save = ($this->getInput('save', '0') === '1');

		$status = null;
		$error = null;
		$backup = null;

		if ($save) {
			if (!function_exists('yaml_emit')) {
				$error = 'Extensão PHP yaml não instalada (yaml_emit). Instale php-yaml.';
				$cfg = $this->loadConfig();
			}
			else {
				$post_default = $this->getInput('default', []);
				$post_clients = $this->getInput('clients', []);

				$cfg = $this->normalizeConfigFromPost($post_default, $post_clients);

				$val = $this->validateConfig($cfg);
				if (!$val['ok']) {
					$error = $val['error'];
				}
				else {
					$yaml_text = yaml_emit($cfg, YAML_UTF8_ENCODING, YAML_LN_BREAK);

					$wr = $this->saveYamlAtomic($yaml_text);
					if (!$wr['ok']) {
						$error = $wr['error'];
					}
					else {
						$status = 'Config salva com sucesso.';
						$backup = $wr['backup'] ?? null;
					}
				}
			}
		}
		else {
			$cfg = $this->loadConfig();
			if (isset($cfg['_error'])) {
				$error = $cfg['_error'];
			}
		}

		// clients dict -> lista pro form
		$clients_list = [];
		if (isset($cfg['clients']) && is_array($cfg['clients'])) {
			foreach ($cfg['clients'] as $name => $c) {
				$clients_list[] = [
					'name' => $name,
					'autoclose' => (bool)($c['autoclose'] ?? false),
					'urgency' => (string)($c['urgency'] ?? ''),
					'impact' => (string)($c['impact'] ?? ''),
					'topdesk' => (array)($c['topdesk'] ?? [])
				];
			}
		}

		$this->setResponse(new CControllerResponseData([
			'title' => _('goDesk - Editar configuração'),
			'path' => $this->config_path,
			'status' => $status,
			'error' => $error,
			'backup' => $backup,
			'default' => (array)($cfg['default'] ?? []),
			'clients' => $clients_list
		]));
	}
}
