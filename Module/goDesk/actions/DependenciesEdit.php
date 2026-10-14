<?php
/**
 * Action: godesk.dependencies.edit
 *
 * Editor do godesk-dependencies.yaml — arquivo separado do
 * godesk-config.yaml, sincronizado pelo mesmo caminho (godesk-client
 * dependencies → POST /dependencies no godesk serve).
 *
 * Schema:
 * dependencies:
 *   - host: "SRV-APP-01"        # nome técnico do host dependente
 *     hostid: "10612"           # preenchido aqui ao salvar (via API)
 *     mode: any                 # any = segura se QUALQUER pai estiver em alerta
 *                               # all = só segura se TODOS estiverem (redundância)
 *     parents:
 *       - host: "SW-CORE-01"
 *         hostid: "10584"
 *         event: "Unavailable by ICMP ping"   # vazio = qualquer problema; aceita *
 *
 * O nome do host é a chave "portável" (export/import entre ambientes); o
 * hostid é sempre recalculado ao salvar/importar, a partir do nome.
 *
 * Só tem efeito pras rules com "dependency: true" (godesk-config.yaml).
 */

namespace Modules\GoDesk\Actions;

use CController;
use CControllerResponseData;

class DependenciesEdit extends CController {

	private string $deps_path = '/etc/zabbix/godesk/godesk-dependencies.yaml';

	public function init(): void {
		$this->disableCsrfValidation();
	}

	protected function checkInput(): bool {
		$fields = [
			'save' => 'in 0,1',
			'import' => 'in 0,1',
			'deps' => 'array',
			'import_yaml' => 'string'
		];

		return $this->validateInput($fields);
	}

	protected function checkPermissions(): bool {
		return (isset(\CWebUser::$data['type']) && \CWebUser::$data['type'] == USER_TYPE_SUPER_ADMIN);
	}

	/**
	 * Lê o arquivo atual. Inexistente = nenhuma dependência ainda (não é
	 * erro). Devolve também o texto cru, usado pelo botão Exportar.
	 */
	private function loadFile(): array {
		if (!file_exists($this->deps_path)) {
			return ['deps' => [], 'raw' => '', 'error' => null];
		}
		if (!is_readable($this->deps_path)) {
			return ['deps' => [], 'raw' => '', 'error' => 'Sem permissão de leitura: '.$this->deps_path];
		}

		$raw = (string)@file_get_contents($this->deps_path);
		$parsed = $this->parseYaml($raw);
		if ($parsed === null) {
			return ['deps' => [], 'raw' => $raw, 'error' => 'YAML inválido em '.$this->deps_path];
		}

		return ['deps' => $parsed, 'raw' => $raw, 'error' => null];
	}

	/**
	 * Parse de um YAML de dependências (arquivo ou importação). null =
	 * inválido; vazio/sem a chave "dependencies" = lista vazia.
	 */
	private function parseYaml(string $text): ?array {
		if (trim($text) === '') {
			return [];
		}

		$parsed = @yaml_parse($text);
		if ($parsed === false || ($parsed !== null && !is_array($parsed))) {
			return null;
		}

		$list = $parsed['dependencies'] ?? [];
		if (!is_array($list)) {
			return null;
		}

		return $this->normalizeDeps($list);
	}

	/**
	 * Normaliza linhas (vindas do POST ou de um YAML): trim, mode padrão
	 * "any", descarta pai sem host e card totalmente vazio (linha nova que
	 * o usuário não preencheu).
	 */
	private function normalizeDeps(array $rows): array {
		$out = [];

		foreach ($rows as $row) {
			if (!is_array($row)) {
				continue;
			}

			$parents = [];
			foreach ((array)($row['parents'] ?? []) as $p) {
				if (!is_array($p)) {
					continue;
				}
				$ph = trim((string)($p['host'] ?? ''));
				if ($ph === '') {
					continue;
				}
				$parents[] = [
					'host' => $ph,
					'hostid' => trim((string)($p['hostid'] ?? '')),
					'event' => trim((string)($p['event'] ?? ''))
				];
			}

			$host = trim((string)($row['host'] ?? ''));
			if ($host === '' && !$parents) {
				continue;
			}

			$mode = strtolower(trim((string)($row['mode'] ?? '')));

			$out[] = [
				'host' => $host,
				'hostid' => trim((string)($row['hostid'] ?? '')),
				'mode' => ($mode === '') ? 'any' : $mode,
				'parents' => $parents
			];
		}

		return $out;
	}

	/**
	 * Recalcula hostid (e o nome técnico canônico) de todos os hosts pelo
	 * nome: primeiro pelo nome técnico, depois pelo nome visível. Hosts que
	 * não existem neste Zabbix ficam com hostid vazio e entram em
	 * $missing — o goDesk tenta de novo pelo nome na hora do alerta.
	 */
	private function resolveHostIds(array $deps, array &$missing): array {
		$names = [];
		foreach ($deps as $d) {
			$names[] = $d['host'];
			foreach ($d['parents'] as $p) {
				$names[] = $p['host'];
			}
		}
		$names = array_values(array_unique(array_filter($names, 'strlen')));

		$found = [];
		$pending = $names;
		foreach (['host', 'name'] as $field) {
			if (!$pending) {
				break;
			}
			$hosts = \API::Host()->get([
				'output' => ['hostid', 'host', 'name'],
				'filter' => [$field => $pending]
			]);
			foreach ((array)$hosts as $h) {
				$found[mb_strtolower($h[$field])] = $h;
			}
			$pending = array_values(array_filter($pending, function ($n) use ($found) {
				return !isset($found[mb_strtolower($n)]);
			}));
		}
		$missing = $pending;

		$apply = function (array $item) use ($found): array {
			$h = $found[mb_strtolower($item['host'])] ?? null;
			$item['hostid'] = $h ? (string)$h['hostid'] : '';
			if ($h) {
				$item['host'] = (string)$h['host'];
			}
			return $item;
		};

		foreach ($deps as $i => $d) {
			$parents = $d['parents'];
			$deps[$i] = $apply($d);
			foreach ($parents as $j => $p) {
				$parents[$j] = $apply($p);
			}
			$deps[$i]['parents'] = $parents;
		}

		return $deps;
	}

	/**
	 * Mesmas regras do config.ParseDependencies no Go (o servidor valida de
	 * novo ao receber): host obrigatório, pelo menos um pai, mode any/all,
	 * sem host repetido, sem depender de si mesmo, sem ciclo.
	 */
	private function validateDeps(array $deps): ?string {
		$seen = [];
		$graph = [];

		foreach ($deps as $i => $d) {
			$n = $i + 1;
			if ($d['host'] === '') {
				return 'Dependência #'.$n.': informe o host dependente.';
			}
			if (!in_array($d['mode'], ['any', 'all'], true)) {
				return 'Host '.$d['host'].': modo inválido "'.$d['mode'].'" (use any ou all).';
			}

			$key = mb_strtolower($d['host']);
			if (isset($seen[$key])) {
				return 'Host '.$d['host'].' cadastrado mais de uma vez — junte os pais num card só.';
			}
			$seen[$key] = true;

			if (!$d['parents']) {
				return 'Host '.$d['host'].': informe pelo menos um host pai.';
			}
			foreach ($d['parents'] as $p) {
				if (mb_strtolower($p['host']) === $key) {
					return 'Host '.$d['host'].' não pode depender de si mesmo.';
				}
				$graph[$key][] = mb_strtolower($p['host']);
			}
		}

		$cycle = $this->findCycle($graph);
		if ($cycle !== null) {
			return 'Dependência circular: '.$cycle;
		}

		return null;
	}

	private function findCycle(array $graph): ?string {
		$state = [];
		$path = [];

		$visit = function (string $n) use (&$visit, &$state, &$path, $graph): ?string {
			$state[$n] = 1;
			$path[] = $n;
			foreach ($graph[$n] ?? [] as $next) {
				if (($state[$next] ?? 0) === 1) {
					$start = array_search($next, $path, true);
					return implode(' → ', array_merge(array_slice($path, $start), [$next]));
				}
				if (($state[$next] ?? 0) === 0) {
					$c = $visit($next);
					if ($c !== null) {
						return $c;
					}
				}
			}
			array_pop($path);
			$state[$n] = 2;
			return null;
		};

		foreach (array_keys($graph) as $n) {
			if (($state[$n] ?? 0) === 0) {
				$c = $visit($n);
				if ($c !== null) {
					return $c;
				}
			}
		}

		return null;
	}

	private function saveYamlAtomic(string $yaml_text): array {
		$dir = dirname($this->deps_path);

		if (file_exists($this->deps_path)) {
			if (!is_writable($this->deps_path)) {
				return ['ok' => false, 'error' => 'Sem permissão de escrita: '.$this->deps_path];
			}
		}
		elseif (!is_writable($dir)) {
			return ['ok' => false, 'error' => 'Sem permissão para criar arquivo em: '.$dir];
		}

		$tmp = $dir.'/'.basename($this->deps_path).'.tmp';
		$bak = $dir.'/'.basename($this->deps_path).'.bak.'.date('Ymd-His');

		if (file_exists($this->deps_path) && !@copy($this->deps_path, $bak)) {
			return ['ok' => false, 'error' => 'Falha ao criar backup em: '.$bak];
		}

		if (@file_put_contents($tmp, $yaml_text, LOCK_EX) === false) {
			return ['ok' => false, 'error' => 'Falha ao escrever tmp: '.$tmp];
		}

		@chmod($tmp, 0640);

		if (!@rename($tmp, $this->deps_path)) {
			@unlink($tmp);
			return ['ok' => false, 'error' => 'Falha ao aplicar arquivo (rename).'];
		}

		return ['ok' => true, 'error' => null, 'backup' => (file_exists($bak) ? $bak : null)];
	}

	/**
	 * Mesmo esquema do ConfigEdit::pushToServer, outro subcomando.
	 * Best-effort: sem godesk-client instalado, retorna null.
	 */
	private function pushToServer(): ?array {
		$bin = '/usr/bin/godesk-client';

		if (!is_file($bin) || !is_executable($bin)) {
			return null;
		}

		$output = [];
		$exit_code = 0;
		exec(escapeshellarg($bin).' dependencies 2>&1', $output, $exit_code);

		return ['ok' => ($exit_code === 0), 'output' => trim(implode("\n", $output))];
	}

	protected function doAction(): void {
		$save = ($this->getInput('save', '0') === '1');
		$import = ($this->getInput('import', '0') === '1');

		$status = null;
		$error = null;
		$warning = null;
		$backup = null;
		$sync_status = null;
		$sync_error = null;

		$file = $this->loadFile();
		$deps = $file['deps'];
		$raw = $file['raw'];
		if ($file['error'] !== null && !$save && !$import) {
			$error = $file['error'];
		}

		if ($save || $import) {
			if (!function_exists('yaml_emit') || !function_exists('yaml_parse')) {
				$error = 'Extensão PHP yaml não instalada. Instale php-yaml.';
			}
			else {
				if ($import) {
					$parsed = $this->parseYaml((string)$this->getInput('import_yaml', ''));
					if ($parsed === null) {
						$error = 'Arquivo importado não é um YAML de dependências válido (esperado a chave "dependencies").';
					}
					else {
						$deps = $parsed;
					}
				}
				else {
					$deps = $this->normalizeDeps((array)$this->getInput('deps', []));
				}

				// resolve antes de validar: troca nome visível pelo técnico,
				// senão "Switch 01" e "SW-01" passariam como hosts diferentes
				// na checagem de duplicado/ciclo
				$missing = [];
				if ($error === null) {
					$deps = $this->resolveHostIds($deps, $missing);
					$error = $this->validateDeps($deps);
				}

				if ($error === null) {
					if ($missing) {
						$warning = 'Hosts não encontrados neste Zabbix (salvos só pelo nome): '.implode(', ', $missing);
					}

					$yaml_text = yaml_emit(['dependencies' => $deps], YAML_UTF8_ENCODING, YAML_LN_BREAK);

					$wr = $this->saveYamlAtomic($yaml_text);
					if (!$wr['ok']) {
						$error = $wr['error'];
					}
					else {
						$raw = $yaml_text;
						$backup = $wr['backup'] ?? null;
						$status = $import
							? 'Dependências importadas e salvas ('.count($deps).' hosts).'
							: 'Dependências salvas com sucesso.';

						$sync = $this->pushToServer();
						if ($sync !== null) {
							if ($sync['ok']) {
								$sync_status = 'Dependências enviadas para o servidor goDesk com sucesso.';
							}
							else {
								$sync_error = 'Falha ao enviar dependências para o servidor goDesk: '.$sync['output'];
							}
						}
					}
				}
			}
		}

		$hosts = \API::Host()->get([
			'output' => ['host', 'name'],
			'sortfield' => 'host'
		]);

		$this->setResponse(new CControllerResponseData([
			'title' => _('goDesk - Dependências'),
			'path' => $this->deps_path,
			'status' => $status,
			'error' => $error,
			'warning' => $warning,
			'backup' => $backup,
			'sync_status' => $sync_status,
			'sync_error' => $sync_error,
			'deps' => $deps,
			'raw' => $raw,
			'hosts' => (array)$hosts
		]));
	}
}
