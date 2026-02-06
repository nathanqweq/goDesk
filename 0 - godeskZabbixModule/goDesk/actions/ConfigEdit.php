<?php

namespace Modules\GoDesk\Actions;

use CController;
use CControllerResponseData;

class ConfigEdit extends CController {

	private string $config_path = '/etc/zabbix/godesk-config.yaml';

	public function init(): void {
		// depois a gente reforça CSRF; por enquanto mantém simples:
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
		// recomendado: só Super Admin
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

		return $parsed;
	}

	private function toBool($v): bool {
		// checkbox pode vir como "1" ou true
		return ($v === 1 || $v === '1' || $v === true || $v === 'true' || $v === 'on');
	}

	private function normalizeConfigFromPost(array $post_default, array $post_clients): array {
		$config = [
			'default' => [
				'urgency' => (string)($post_default['urgency'] ?? ''),
				'impact' => (string)($post_default['impact'] ?? ''),
				'autoclose' => $this->toBool($post_default['autoclose'] ?? false),
				'tags' => [
					'contract' => (string)($post_default['tags']['contract'] ?? ''),
					'oper_group' => (string)($post_default['tags']['oper_group'] ?? ''),
					'main_caller' => (string)($post_default['tags']['main_caller'] ?? ''),
					'secundary_caller' => (string)($post_default['tags']['secundary_caller'] ?? '')
				]
			],
			'clients' => []
		];

		// clients vem como lista: clients[0][name], clients[0][urgency], etc.
		foreach ($post_clients as $row) {
			$name = trim((string)($row['name'] ?? ''));
			if ($name === '') {
				continue;
			}

			$config['clients'][$name] = [
				'autoclose' => $this->toBool($row['autoclose'] ?? false),
				'urgency' => (string)($row['urgency'] ?? ''),
				'impact' => (string)($row['impact'] ?? ''),
				'tags' => [
					'contract' => (string)($row['tags']['contract'] ?? ''),
					'oper_group' => (string)($row['tags']['oper_group'] ?? ''),
					'main_caller' => (string)($row['tags']['main_caller'] ?? ''),
					'secundary_caller' => (string)($row['tags']['secundary_caller'] ?? '')
				]
			];
		}

		return $config;
	}

	private function validateConfig(array $cfg): array {
		if (!isset($cfg['default']) || !isset($cfg['clients'])) {
			return ['ok' => false, 'error' => 'Config inválida: faltam as chaves default/clients.'];
		}
		// checks mínimos
		if (!isset($cfg['default']['tags']) || !is_array($cfg['default']['tags'])) {
			return ['ok' => false, 'error' => 'Config inválida: default.tags ausente.'];
		}
		return ['ok' => true, 'error' => null];
	}

	private function saveYamlAtomic(string $yaml_text): array {
		if (!file_exists($this->config_path)) {
			// se não existe, precisa permissão de escrita no diretório
			$dir = dirname($this->config_path);
			if (!is_writable($dir)) {
				return ['ok' => false, 'error' => 'Sem permissão para criar arquivo em: '.$dir];
			}
		}
		else if (!is_writable($this->config_path)) {
			return ['ok' => false, 'error' => 'Sem permissão de escrita: '.$this->config_path];
		}

		$dir = dirname($this->config_path);
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

		// transforma clients dict em lista pro form ficar fácil
		$clients_list = [];
		if (isset($cfg['clients']) && is_array($cfg['clients'])) {
			foreach ($cfg['clients'] as $name => $c) {
				$clients_list[] = [
					'name' => $name,
					'autoclose' => (bool)($c['autoclose'] ?? false),
					'urgency' => (string)($c['urgency'] ?? ''),
					'impact' => (string)($c['impact'] ?? ''),
					'tags' => (array)($c['tags'] ?? [])
				];
			}
		}

		$this->setResponse(new CControllerResponseData([
			'title' => _('goDesk - Editar config'),
			'path' => $this->config_path,
			'status' => $status,
			'error' => $error,
			'backup' => $backup,
			'default' => (array)($cfg['default'] ?? []),
			'clients' => $clients_list
		]));
	}
}
