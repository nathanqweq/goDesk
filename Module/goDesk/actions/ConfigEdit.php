<?php
/**
 * Action: godesk.config.edit
 *
 * Editor de configuração do goDesk.
 *
 * Schema (formato novo — clientes nomeados + rules referenciando eles):
 * default:
 *   client, urgency, impact, priority, autoclose
 *   topdesk: { contract, operator, oper_group, main_caller, secundary_caller, sla, category, sub_category, call_type, send_more_info, more_info_text, adicional_cresol, send_email, email_to, email_cc, once_per_day }
 *
 * clients:
 *   <NOME_DO_CLIENTE>:
 *     topdesk: { ... compartilhado por todas as rules desse cliente ... }
 *
 * rules:
 *   <RULE_NAME>:
 *     client: "NOME_DO_CLIENTE"   # referencia clients.<NOME_DO_CLIENTE>
 *     urgency, impact, priority, autoclose
 *     custom_status: bool         # sempre da própria rule, nunca do cliente
 *     status_open: ""             # id do processingStatus no TopDesk, usado
 *                                  # na abertura do chamado quando custom_status=true
 *     status_update: ""           # id do processingStatus no TopDesk, usado
 *                                  # ao atualizar o chamado quando custom_status=true
 *     topdesk: { ... só overrides de texto (ex: more_info_text) ... }
 *
 * Formato antigo (clients: {RULE_NAME: {...topdesk completo...}}, sem
 * "rules") continua sendo lido normalmente — ver loadConfig(). A partir do
 * primeiro save por este módulo, o arquivo sempre é regravado no formato
 * novo.
 */

namespace Modules\GoDesk\Actions;

use CController;
use CControllerResponseData;

class ConfigEdit extends CController {

	private string $config_path = '/etc/zabbix/godesk/godesk-config.yaml';

	public function init(): void {
		$this->disableCsrfValidation();
	}

	protected function checkInput(): bool {
		$fields = [
			'save' => 'in 0,1',
			'default' => 'array',
			'clients' => 'array',
			'named_clients' => 'array'
		];

		return $this->validateInput($fields);
	}

	protected function checkPermissions(): bool {
		return (isset(\CWebUser::$data['type']) && \CWebUser::$data['type'] == USER_TYPE_SUPER_ADMIN);
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
			return ['default' => [], 'rules' => [], 'named_clients' => []];
		}

		if (!is_readable($this->config_path)) {
			return ['default' => [], 'rules' => [], 'named_clients' => [], '_error' => 'Sem permissão de leitura: '.$this->config_path];
		}

		if (!function_exists('yaml_parse_file')) {
			return ['default' => [], 'rules' => [], 'named_clients' => [], '_error' => 'Extensão PHP yaml não instalada (yaml_parse_file).'];
		}

		$parsed = @yaml_parse_file($this->config_path);

		if ($parsed === false || !is_array($parsed)) {
			return ['default' => [], 'rules' => [], 'named_clients' => [], '_error' => 'YAML inválido ou vazio.'];
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
			// formato antigo: "clients" é a lista de rules (chave = rule_name,
			// topdesk completo, sem cliente nomeado nenhum ainda)
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

	private function toBool($v): bool {
		return ($v === 1 || $v === '1' || $v === true || $v === 'true' || $v === 'on');
	}

	private function normalizeNullString($v): string {
		return (string)$v;
	}

	private function normalizeTopdeskFromPost(array $td): array {
		return [
			'contract' => (string)($td['contract'] ?? ''),
			'operator' => (string)($td['operator'] ?? ''),
			'oper_group' => (string)($td['oper_group'] ?? ''),
			'main_caller' => (string)($td['main_caller'] ?? ''),
			'secundary_caller' => $this->normalizeNullString($td['secundary_caller'] ?? ''),
			'sla' => (string)($td['sla'] ?? ''),
			'category' => (string)($td['category'] ?? ''),
			'sub_category' => (string)($td['sub_category'] ?? ''),
			'call_type' => (string)($td['call_type'] ?? ''),
			'send_more_info' => $this->toBool($td['send_more_info'] ?? false),
			'more_info_text' => (string)($td['more_info_text'] ?? ''),
			'adicional_cresol' => $this->toBool($td['adicional_cresol'] ?? false),
			'send_email' => $this->toBool($td['send_email'] ?? false),
			'email_to' => (string)($td['email_to'] ?? ''),
			'email_cc' => (string)($td['email_cc'] ?? ''),
			'once_per_day' => $this->toBool($td['once_per_day'] ?? false)
		];
	}

	/**
	 * Monta o config a partir do POST. Sempre grava no formato novo
	 * (clients = nomeados, rules = por rule_name) — é assim que a migração
	 * do formato antigo acontece: no primeiro save pela UI, o arquivo
	 * inteiro é reescrito no formato novo.
	 */
	private function normalizeConfigFromPost(array $post_default, array $post_named_clients, array $post_rules): array {
		$def_td = (array)($post_default['topdesk'] ?? []);

		$config = [
			'default' => [
				'client' => (string)($post_default['client'] ?? ''),
				'urgency' => (string)($post_default['urgency'] ?? ''),
				'impact' => (string)($post_default['impact'] ?? ''),
				'priority' => (string)($post_default['priority'] ?? ''),
				'autoclose' => $this->toBool($post_default['autoclose'] ?? false),
				'topdesk' => $this->normalizeTopdeskFromPost($def_td)
			],
			'clients' => [],
			'rules' => []
		];

		foreach ($post_named_clients as $row) {
			$name = trim((string)($row['name'] ?? ''));
			if ($name === '') {
				continue;
			}

			$td = (array)($row['topdesk'] ?? []);
			$config['clients'][$name] = [
				'topdesk' => $this->normalizeTopdeskFromPost($td)
			];
		}

		foreach ($post_rules as $row) {
			$rule_name = trim((string)($row['rule_name'] ?? ''));
			if ($rule_name === '') {
				continue;
			}

			$td = (array)($row['topdesk'] ?? []);
			$config['rules'][$rule_name] = [
				'client' => (string)($row['client'] ?? ''),
				'autoclose' => $this->toBool($row['autoclose'] ?? false),
				'urgency' => (string)($row['urgency'] ?? ''),
				'impact' => (string)($row['impact'] ?? ''),
				'priority' => (string)($row['priority'] ?? ''),
				'custom_status' => $this->toBool($row['custom_status'] ?? false),
				'status_open' => (string)($row['status_open'] ?? ''),
				'status_update' => (string)($row['status_update'] ?? ''),
				'topdesk' => $this->normalizeTopdeskFromPost($td)
			];
		}

		return $config;
	}

	private function validateConfig(array $cfg): array {
		if (!isset($cfg['default']) || !isset($cfg['clients']) || !isset($cfg['rules'])) {
			return ['ok' => false, 'error' => 'Config inválida: faltam as chaves default/clients/rules.'];
		}

		if (!isset($cfg['default']['topdesk']) || !is_array($cfg['default']['topdesk'])) {
			return ['ok' => false, 'error' => 'Config inválida: default.topdesk ausente.'];
		}

		foreach (['contract','operator','oper_group','main_caller','secundary_caller','sla','category','sub_category','call_type','send_more_info','more_info_text','adicional_cresol','send_email','email_to','email_cc','once_per_day'] as $k) {
			if (!array_key_exists($k, $cfg['default']['topdesk'])) {
				return ['ok' => false, 'error' => 'Config inválida: default.topdesk.'.$k.' ausente.'];
			}
		}

		foreach ($cfg['clients'] as $name => $c) {
			if (!isset($c['topdesk']) || !is_array($c['topdesk'])) {
				return ['ok' => false, 'error' => 'Config inválida: clients.'.$name.'.topdesk ausente.'];
			}
		}

		foreach ($cfg['rules'] as $rule => $r) {
			if (!isset($r['topdesk']) || !is_array($r['topdesk'])) {
				return ['ok' => false, 'error' => 'Config inválida: rules.'.$rule.'.topdesk ausente.'];
			}
		}

		return ['ok' => true, 'error' => null];
	}

	/**
	 * Envia o YAML recém-salvo para o godesk-client, que valida e repassa
	 * pro godesk serve remoto (usado quando o servidor que roda o godesk
	 * está em host diferente deste frontend). Best-effort: se o pacote
	 * godesk-client não estiver instalado neste host, retorna null e o
	 * fluxo de save local continua igual ao de antes.
	 */
	private function pushToServer(): ?array {
		$bin = '/usr/bin/godesk-client';

		if (!is_file($bin) || !is_executable($bin)) {
			return null;
		}

		$output = [];
		$exit_code = 0;
		exec(escapeshellarg($bin).' 2>&1', $output, $exit_code);

		return ['ok' => ($exit_code === 0), 'output' => trim(implode("\n", $output))];
	}

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
		$sync_status = null;
		$sync_error = null;

		if ($save) {
			if (!function_exists('yaml_emit')) {
				$error = 'Extensão PHP yaml não instalada (yaml_emit). Instale php-yaml.';
				$cfg = $this->loadConfig();
			}
			else {
				$post_default = $this->getInput('default', []);
				$post_named_clients = $this->getInput('named_clients', []);
				$post_rules = $this->getInput('clients', []);

				$cfg = $this->normalizeConfigFromPost($post_default, $post_named_clients, $post_rules);

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

						$sync = $this->pushToServer();
						if ($sync !== null) {
							if ($sync['ok']) {
								$sync_status = 'Config enviada para o servidor goDesk com sucesso.';
							}
							else {
								$sync_error = 'Falha ao enviar config para o servidor goDesk: '.$sync['output'];
							}
						}
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

		$rules_list = [];
		if (isset($cfg['rules']) && is_array($cfg['rules'])) {
			foreach ($cfg['rules'] as $rule_name => $r) {
				$rules_list[] = [
					'rule_name' => $rule_name,
					'client' => (string)($r['client'] ?? ''),
					'autoclose' => (bool)($r['autoclose'] ?? false),
					'urgency' => (string)($r['urgency'] ?? ''),
					'impact' => (string)($r['impact'] ?? ''),
					'priority' => (string)($r['priority'] ?? ''),
					'custom_status' => (bool)($r['custom_status'] ?? false),
					'status_open' => (string)($r['status_open'] ?? ''),
					'status_update' => (string)($r['status_update'] ?? ''),
					'topdesk' => (array)($r['topdesk'] ?? [])
				];
			}
		}

		$named_clients_list = [];
		if (isset($cfg['named_clients']) && is_array($cfg['named_clients'])) {
			foreach ($cfg['named_clients'] as $name => $c) {
				$named_clients_list[] = [
					'name' => $name,
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
			'sync_status' => $sync_status,
			'sync_error' => $sync_error,
			'default' => (array)($cfg['default'] ?? []),
			'named_clients' => $named_clients_list,
			'rules' => $rules_list
		]));
	}
}
