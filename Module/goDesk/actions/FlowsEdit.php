<?php
/**
 * Action: godesk.flows.edit
 *
 * Editor visual do godesk-flows.yaml: fluxos de decisão das rules (ver
 * internal/config/flows.go). A rule aponta pra um fluxo (campo "Fluxo" em
 * Config (editar)) e o fluxo decide na hora do alerta — condições sobre o
 * evento, campos do chamado, instrução pro atendimento, abrir ou não.
 *
 * O editor (assets/js/godesk-flows.js) trabalha em JSON e manda tudo num
 * campo só (flows_json). Aqui o JSON é normalizado, validado pelo próprio
 * goDesk (godesk-client flows-check, a mesma validação do servidor),
 * gravado e sincronizado (godesk-client flows → POST /flows).
 *
 * O arquivo é gravado em JSON indentado (JSON é YAML válido): evita as
 * pegadinhas do YAML 1.1 do yaml_parse (chave y, valores on/off/yes/no
 * virando booleano) e continua legível.
 */

namespace Modules\GoDesk\Actions;

use CController;
use CControllerResponseData;

class FlowsEdit extends CController {

	protected string $path = '/etc/zabbix/godesk/godesk-flows.yaml';
	private string $client_bin = '/usr/bin/godesk-client';

	public function init(): void {
		$this->disableCsrfValidation();
	}

	protected function checkInput(): bool {
		return $this->validateInput([
			'save' => 'in 0,1',
			'import' => 'in 0,1',
			'flows_json' => 'string',
			'import_yaml' => 'string'
		]);
	}

	protected function checkPermissions(): bool {
		return ($this->getUserType() == USER_TYPE_SUPER_ADMIN);
	}

	protected function loadFile(): array {
		if (!file_exists($this->path)) {
			return ['data' => ['calendars' => [], 'flows' => []], 'raw' => '', 'error' => null];
		}
		if (!is_readable($this->path)) {
			return ['data' => ['calendars' => [], 'flows' => []], 'raw' => '', 'error' => 'Sem permissão de leitura: '.$this->path];
		}
		$raw = (string)@file_get_contents($this->path);
		$data = $this->parseText($raw);
		if ($data === null) {
			return ['data' => ['calendars' => [], 'flows' => []], 'raw' => $raw, 'error' => 'Arquivo inválido: '.$this->path];
		}
		return ['data' => $data, 'raw' => $raw, 'error' => null];
	}

	/** JSON primeiro (formato gravado por esta página), YAML depois. null = inválido. */
	protected function parseText(string $text): ?array {
		if (trim($text) === '') {
			return ['calendars' => [], 'flows' => []];
		}
		$body = preg_replace('/^\s*#.*$/m', '', $text);
		$parsed = json_decode((string)$body, true);
		if (!is_array($parsed)) {
			if (!function_exists('yaml_parse')) {
				return null;
			}
			$parsed = @yaml_parse($text);
		}
		if (!is_array($parsed)) {
			return null;
		}
		return $this->normalize($parsed);
	}

	/** Escalar do YAML 1.1 de volta pra texto (true → "true", 1 → "1"). */
	private static function str($v): string {
		if (is_bool($v)) {
			return $v ? 'true' : 'false';
		}
		if (is_array($v) || is_object($v)) {
			return '';
		}
		return trim((string)$v);
	}

	/**
	 * Normaliza a estrutura (editor ou arquivo importado): só os campos
	 * conhecidos, tudo string/int, listas reindexadas. A validação de verdade
	 * fica com o goDesk (flows-check).
	 */
	protected function normalize(array $in): array {
		$cals = [];
		foreach ((array)($in['calendars'] ?? []) as $name => $c) {
			$name = self::str($name);
			if ($name === '' || !is_array($c)) {
				continue;
			}
			$cals[$name] = [
				'days' => array_values(array_filter(array_map([self::class, 'str'], (array)($c['days'] ?? [])), 'strlen')),
				'start' => self::str($c['start'] ?? ''),
				'end' => self::str($c['end'] ?? ''),
				'holidays' => array_values(array_filter(array_map([self::class, 'str'], (array)($c['holidays'] ?? [])), 'strlen'))
			];
		}

		$flows = [];
		foreach ((array)($in['flows'] ?? []) as $f) {
			if (!is_array($f)) {
				continue;
			}
			$nodes = [];
			foreach ((array)($f['nodes'] ?? []) as $n) {
				if (!is_array($n)) {
					continue;
				}
				$node = [
					'id' => self::str($n['id'] ?? ''),
					'type' => strtolower(self::str($n['type'] ?? '')),
					'label' => self::str($n['label'] ?? ''),
					'x' => (int)round((float)($n['x'] ?? 0)),
					// chave y sem aspas vira booleano no YAML 1.1 → $n[1]
					'y' => (int)round((float)($n['y'] ?? $n[1] ?? 0))
				];
				switch ($node['type']) {
					case 'if':
						$node['match'] = (self::str($n['match'] ?? '') === 'any') ? 'any' : 'all';
						$node['conditions'] = [];
						foreach ((array)($n['conditions'] ?? []) as $c) {
							if (is_array($c)) {
								$node['conditions'][] = [
									'field' => self::str($c['field'] ?? ''),
									'op' => self::str($c['op'] ?? 'eq'),
									'value' => self::str($c['value'] ?? '')
								];
							}
						}
						break;
					case 'switch':
						$node['field'] = self::str($n['field'] ?? '');
						$node['cases'] = [];
						foreach ((array)($n['cases'] ?? []) as $c) {
							$node['cases'][] = ['value' => self::str(is_array($c) ? ($c['value'] ?? '') : $c)];
						}
						break;
					case 'set':
						$node['set'] = [];
						foreach ((array)($n['set'] ?? []) as $a) {
							if (is_array($a)) {
								$node['set'][] = ['field' => self::str($a['field'] ?? ''), 'value' => self::str($a['value'] ?? '')];
							}
						}
						break;
					case 'end':
						$node['action'] = (self::str($n['action'] ?? '') === 'skip') ? 'skip' : 'open';
						$node['comment'] = self::str($n['comment'] ?? '');
						break;
				}
				$nodes[] = $node;
			}

			$links = [];
			foreach ((array)($f['links'] ?? []) as $l) {
				if (!is_array($l)) {
					continue;
				}
				$link = ['from' => self::str($l['from'] ?? ''), 'to' => self::str($l['to'] ?? '')];
				$port = self::str($l['port'] ?? '');
				if ($port !== '' && $port !== 'next') {
					$link['port'] = $port;
				}
				if ($link['from'] !== '' && $link['to'] !== '') {
					$links[] = $link;
				}
			}

			$flows[] = [
				'name' => self::str($f['name'] ?? ''),
				'description' => self::str($f['description'] ?? ''),
				'nodes' => $nodes,
				'links' => $links
			];
		}

		return ['calendars' => $cals, 'flows' => $flows];
	}

	/** Texto gravado: cabeçalho + JSON indentado (válido como YAML). */
	protected function emit(array $data): string {
		$out = $data;
		if (!$out['calendars']) {
			$out['calendars'] = new \stdClass();
		}
		return "# goDesk — fluxos de decisão das rules (godesk-flows.yaml)\n".
			"# Gerado pela página Alerts → goDesk → Fluxos. JSON é YAML válido.\n".
			json_encode($out, JSON_PRETTY_PRINT | JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES | JSON_INVALID_UTF8_SUBSTITUTE)."\n";
	}

	/**
	 * Valida com o goDesk (mesma regra do servidor). Sem o binário, só o
	 * básico — o servidor valida de novo ao receber.
	 */
	protected function validate(string $text, array $data): ?string {
		// o módulo e o godesk-client vêm no mesmo pacote: o flows-check existe
		if (is_file($this->client_bin) && is_executable($this->client_bin)) {
			$tmp = tempnam(sys_get_temp_dir(), 'godesk-flows-');
			if ($tmp !== false && file_put_contents($tmp, $text) !== false) {
				$output = [];
				$code = 0;
				exec(escapeshellarg($this->client_bin).' flows-check '.escapeshellarg($tmp).' 2>&1', $output, $code);
				@unlink($tmp);
				return ($code === 0) ? null : ('Fluxo inválido: '.trim(implode("\n", $output)));
			}
		}

		$names = [];
		foreach ($data['flows'] as $i => $f) {
			if ($f['name'] === '') {
				return 'Fluxo #'.($i + 1).': informe o nome.';
			}
			$k = mb_strtolower($f['name']);
			if (isset($names[$k])) {
				return 'Fluxo '.$f['name'].' cadastrado mais de uma vez.';
			}
			$names[$k] = true;
			$starts = count(array_filter($f['nodes'], fn($n) => $n['type'] === 'start'));
			if ($starts !== 1) {
				return 'Fluxo '.$f['name'].': precisa de exatamente 1 bloco de início.';
			}
		}
		return null;
	}

	private function saveAtomic(string $text): array {
		$dir = dirname($this->path);
		if (file_exists($this->path) ? !is_writable($this->path) : !is_writable($dir)) {
			return ['ok' => false, 'error' => 'Sem permissão de escrita em '.$this->path];
		}
		$tmp = $dir.'/'.basename($this->path).'.tmp';
		$bak = $dir.'/'.basename($this->path).'.bak.'.date('Ymd-His');
		if (file_exists($this->path) && !@copy($this->path, $bak)) {
			return ['ok' => false, 'error' => 'Falha ao criar backup em: '.$bak];
		}
		if (@file_put_contents($tmp, $text, LOCK_EX) === false) {
			return ['ok' => false, 'error' => 'Falha ao escrever tmp: '.$tmp];
		}
		@chmod($tmp, 0640);
		if (!@rename($tmp, $this->path)) {
			@unlink($tmp);
			return ['ok' => false, 'error' => 'Falha ao aplicar arquivo (rename).'];
		}
		return ['ok' => true, 'backup' => file_exists($bak) ? $bak : null];
	}

	private function pushToServer(): ?array {
		if (!is_file($this->client_bin) || !is_executable($this->client_bin)) {
			return null;
		}
		$output = [];
		$code = 0;
		exec(escapeshellarg($this->client_bin).' flows 2>&1', $output, $code);
		return ['ok' => ($code === 0), 'output' => trim(implode("\n", $output))];
	}

	/** Rules que usam cada fluxo (pra mostrar no editor). */
	private function rulesByFlow(): array {
		$path = '/etc/zabbix/godesk/godesk-config.yaml';
		if (!function_exists('yaml_parse') || !is_readable($path)) {
			return [];
		}
		$parsed = @yaml_parse((string)@file_get_contents($path));
		$rules = is_array($parsed['rules'] ?? null) ? $parsed['rules'] : (is_array($parsed['clients'] ?? null) ? $parsed['clients'] : []);
		$out = [];
		foreach ($rules as $name => $r) {
			$flow = is_array($r) ? trim((string)($r['flow'] ?? '')) : '';
			if ($flow !== '') {
				$out[mb_strtolower($flow)][] = [
					'rule' => (string)$name,
					'mode' => (strtolower((string)($r['flow_mode'] ?? '')) === 'shadow') ? 'shadow' : 'on'
				];
			}
		}
		return $out;
	}

	protected function doAction(): void {
		$save = ($this->getInput('save', '0') === '1');
		$import = ($this->getInput('import', '0') === '1');
		$status = $error = $backup = $sync_status = $sync_error = null;

		$file = $this->loadFile();
		$data = $file['data'];
		$raw = $file['raw'];
		if ($file['error'] !== null && !$save && !$import) {
			$error = $file['error'];
		}

		if ($save || $import) {
			$incoming = null;
			if ($import) {
				$incoming = $this->parseText((string)$this->getInput('import_yaml', ''));
				if ($incoming === null) {
					$error = 'Arquivo importado não é um godesk-flows.yaml válido.';
				}
			}
			else {
				$decoded = json_decode((string)$this->getInput('flows_json', ''), true);
				if (!is_array($decoded)) {
					$error = 'Dados do editor inválidos (JSON).';
				}
				else {
					$incoming = $this->normalize($decoded);
				}
			}

			if ($incoming !== null) {
				// mesmo com erro, devolve o que veio pro editor não perder o desenho
				$data = $incoming;
				$text = $this->emit($data);
				$error = $this->validate($text, $data);

				if ($error === null) {
					$wr = $this->saveAtomic($text);
					if (!$wr['ok']) {
						$error = $wr['error'];
					}
					else {
						$raw = $text;
						$backup = $wr['backup'];
						$status = $import ? 'Fluxos importados e salvos.' : 'Fluxos salvos com sucesso.';
						$sync = $this->pushToServer();
						if ($sync !== null) {
							if ($sync['ok']) {
								$sync_status = 'Fluxos enviados para o servidor goDesk com sucesso.';
							}
							else {
								$sync_error = 'Falha ao enviar fluxos para o servidor goDesk: '.$sync['output'];
							}
						}
					}
				}
			}
		}

		$this->setResponse(new CControllerResponseData([
			'title' => _('goDesk - Fluxos'),
			'path' => $this->path,
			'status' => $status,
			'error' => $error,
			'backup' => $backup,
			'sync_status' => $sync_status,
			'sync_error' => $sync_error,
			'data' => $data,
			'raw' => $raw,
			'rules_by_flow' => $this->rulesByFlow()
		]));
	}
}
