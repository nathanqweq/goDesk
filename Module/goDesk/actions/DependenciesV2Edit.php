<?php
/**
 * Action: godesk.topology.edit
 *
 * Editor visual do godesk-dependencies-v2.yaml (dependência v2: arquitetura
 * por localidade). Roda em paralelo à v1 (godesk.dependencies.edit); qual
 * versão vale é escolhido por rule (dependency_version: v1 | v2 | shadow).
 *
 * Schema (ver internal/config/dependencies_v2.go):
 * sites:
 *   - name: "UHCC"
 *     root: "ZABBIX"                         # ponto de entrada do monitoramento
 *     down_event: "*Unavailable by ICMP ping*"
 *     status:
 *       host: "UHCC - Core Status"           # host agregado da localidade
 *       event: "Powerplant Down"             # com ele ativo, só o status abre chamado
 *     nodes:
 *       - {host: "SD_UHCC_ADM", rack: "1", x: 120, y: 80}
 *     links:
 *       - ["ZABBIX", "SD_UHCC_ADM"]
 *
 * O editor (assets/js/godesk-topology.js) trabalha em JSON e manda tudo num
 * campo só (topology_json); aqui o JSON é normalizado, validado com as
 * mesmas regras do Go, gravado como YAML e sincronizado com o servidor
 * (godesk-client dependencies-v2 → POST /dependencies-v2).
 */

namespace Modules\GoDesk\Actions;

use CController;
use CControllerResponseData;

class DependenciesV2Edit extends CController {

	private string $path = '/etc/zabbix/godesk/godesk-dependencies-v2.yaml';

	private const DEFAULT_ROOT = 'ZABBIX';
	private const DEFAULT_DOWN_EVENT = '*Unavailable by ICMP ping*';

	public function init(): void {
		$this->disableCsrfValidation();
	}

	protected function checkInput(): bool {
		return $this->validateInput([
			'save' => 'in 0,1',
			'import' => 'in 0,1',
			'topology_json' => 'string',
			'import_yaml' => 'string'
		]);
	}

	protected function checkPermissions(): bool {
		return (isset(\CWebUser::$data['type']) && \CWebUser::$data['type'] == USER_TYPE_SUPER_ADMIN);
	}

	private function loadFile(): array {
		if (!file_exists($this->path)) {
			return ['sites' => [], 'raw' => '', 'error' => null];
		}
		if (!is_readable($this->path)) {
			return ['sites' => [], 'raw' => '', 'error' => 'Sem permissão de leitura: '.$this->path];
		}

		$raw = (string)@file_get_contents($this->path);
		$sites = $this->parseYaml($raw);
		if ($sites === null) {
			return ['sites' => [], 'raw' => $raw, 'error' => 'YAML inválido em '.$this->path];
		}
		return ['sites' => $sites, 'raw' => $raw, 'error' => null];
	}

	/** null = inválido; vazio/sem "sites" = lista vazia. */
	private function parseYaml(string $text): ?array {
		if (trim($text) === '') {
			return [];
		}
		$parsed = @yaml_parse($text);
		if ($parsed === false || ($parsed !== null && !is_array($parsed))) {
			return null;
		}
		$list = $parsed['sites'] ?? [];
		return is_array($list) ? $this->normalizeSites($list) : null;
	}

	/**
	 * Normaliza (JSON do editor ou YAML importado): trim, defaults, x/y
	 * inteiros, descarta nó sem host e link incompleto, junta links
	 * repetidos (A-B e B-A são o mesmo cabo).
	 */
	private function normalizeSites(array $rows): array {
		$out = [];

		foreach ($rows as $s) {
			if (!is_array($s)) {
				continue;
			}

			$root = trim((string)($s['root'] ?? ''));
			$status = is_array($s['status'] ?? null) ? $s['status'] : [];

			$nodes = [];
			foreach ((array)($s['nodes'] ?? []) as $n) {
				if (!is_array($n)) {
					continue;
				}
				$host = trim((string)($n['host'] ?? ''));
				if ($host === '') {
					continue;
				}
				$nodes[] = [
					'host' => $host,
					'rack' => trim((string)($n['rack'] ?? '')),
					'x' => (int)round((float)($n['x'] ?? 0)),
					'y' => (int)round((float)($n['y'] ?? 0))
				];
			}

			$links = [];
			$seen = [];
			foreach ((array)($s['links'] ?? []) as $l) {
				if (!is_array($l)) {
					continue;
				}
				$l = array_values($l);
				$a = trim((string)($l[0] ?? ''));
				$b = trim((string)($l[1] ?? ''));
				if ($a === '' || $b === '') {
					continue;
				}
				$pair = [mb_strtolower($a), mb_strtolower($b)];
				sort($pair);
				$key = implode("\n", $pair);
				if (isset($seen[$key])) {
					continue;
				}
				$seen[$key] = true;
				$links[] = [$a, $b];
			}

			$name = trim((string)($s['name'] ?? ''));
			if ($name === '' && !$nodes && !$links) {
				continue;
			}

			$out[] = [
				'name' => $name,
				'root' => ($root === '') ? self::DEFAULT_ROOT : $root,
				'down_event' => trim((string)($s['down_event'] ?? '')) ?: self::DEFAULT_DOWN_EVENT,
				'status' => [
					'host' => trim((string)($status['host'] ?? '')),
					'event' => trim((string)($status['event'] ?? ''))
				],
				'nodes' => $nodes,
				'links' => $links
			];
		}

		return $out;
	}

	/** Mesmas regras do config.ParseDependenciesV2 (o servidor valida de novo). */
	private function validateSites(array $sites): ?string {
		$names = [];
		$owner = [];

		$claim = function (string $site, string $host) use (&$owner): ?string {
			$k = mb_strtolower($host);
			if (isset($owner[$k]) && $owner[$k] !== $site) {
				return 'Host '.$host.' está nas localidades '.$owner[$k].' e '.$site.'.';
			}
			$owner[$k] = $site;
			return null;
		};

		foreach ($sites as $i => $s) {
			$name = $s['name'];
			if ($name === '') {
				return 'Localidade #'.($i + 1).': informe o nome.';
			}
			if (isset($names[mb_strtolower($name)])) {
				return 'Localidade '.$name.' cadastrada mais de uma vez.';
			}
			$names[mb_strtolower($name)] = true;

			$root = mb_strtolower($s['root']);
			$status = mb_strtolower($s['status']['host']);

			if ($status !== '') {
				if ($status === $root) {
					return 'Localidade '.$name.': o host de status não pode ser o root.';
				}
				if (($e = $claim($name, $s['status']['host'])) !== null) {
					return $e;
				}
			}

			$seen = [];
			foreach ($s['nodes'] as $n) {
				$k = mb_strtolower($n['host']);
				if (isset($seen[$k])) {
					return 'Localidade '.$name.': nó '.$n['host'].' repetido.';
				}
				$seen[$k] = true;
				if ($k !== $root && ($e = $claim($name, $n['host'])) !== null) {
					return $e;
				}
			}

			$root_linked = false;
			foreach ($s['links'] as $j => $l) {
				if (mb_strtolower($l[0]) === mb_strtolower($l[1])) {
					return 'Localidade '.$name.': link #'.($j + 1).' liga '.$l[0].' nele mesmo.';
				}
				foreach ($l as $h) {
					$k = mb_strtolower($h);
					if ($k === $root) {
						$root_linked = true;
						continue;
					}
					if ($status !== '' && $k === $status) {
						return 'Localidade '.$name.': o host de status ('.$h.') não entra nos links — ele é avaliado à parte.';
					}
					if (($e = $claim($name, $h)) !== null) {
						return $e;
					}
				}
			}

			if ($s['links'] && !$root_linked) {
				return 'Localidade '.$name.': nenhum link sai do root '.$s['root'].' — ligue o root aos equipamentos de entrada (ex: os SDs).';
			}
		}

		return null;
	}

	/** Hosts citados que não existem neste Zabbix (só aviso: a v2 resolve pelo nome na hora do alerta). */
	private function missingHosts(array $sites): array {
		$names = [];
		foreach ($sites as $s) {
			$root = mb_strtolower($s['root']);
			if ($s['status']['host'] !== '') {
				$names[] = $s['status']['host'];
			}
			foreach ($s['nodes'] as $n) {
				if (mb_strtolower($n['host']) !== $root) {
					$names[] = $n['host'];
				}
			}
			foreach ($s['links'] as $l) {
				foreach ($l as $h) {
					if (mb_strtolower($h) !== $root) {
						$names[] = $h;
					}
				}
			}
		}
		$names = array_values(array_unique($names));
		if (!$names) {
			return [];
		}

		$found = [];
		foreach (['host', 'name'] as $field) {
			$hosts = \API::Host()->get(['output' => ['host', 'name'], 'filter' => [$field => $names]]);
			foreach ((array)$hosts as $h) {
				$found[mb_strtolower($h[$field])] = true;
			}
		}

		return array_values(array_filter($names, function ($n) use ($found) {
			return !isset($found[mb_strtolower($n)]);
		}));
	}

	/** Strings como JSON: um escalar JSON é um escalar YAML válido (aspas duplas). */
	private static function q(string $s): string {
		return (string)json_encode($s, JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES | JSON_INVALID_UTF8_SUBSTITUTE);
	}

	/**
	 * YAML legível à mão (nós e links em uma linha cada) em vez do
	 * yaml_emit, que quebraria cada link em três linhas.
	 */
	private function emitYaml(array $sites): string {
		$y = "# goDesk — dependência v2 (arquitetura por localidade)\n";
		$y .= "# Gerado pela página Alerts → goDesk → Dependências v2.\n";
		if (!$sites) {
			return $y."sites: []\n";
		}

		$y .= "sites:\n";
		foreach ($sites as $s) {
			$y .= '  - name: '.self::q($s['name'])."\n";
			$y .= '    root: '.self::q($s['root'])."\n";
			$y .= '    down_event: '.self::q($s['down_event'])."\n";
			$y .= "    status:\n";
			$y .= '      host: '.self::q($s['status']['host'])."\n";
			$y .= '      event: '.self::q($s['status']['event'])."\n";

			if ($s['nodes']) {
				$y .= "    nodes:\n";
				foreach ($s['nodes'] as $n) {
					$parts = ['host: '.self::q($n['host'])];
					if ($n['rack'] !== '') {
						$parts[] = 'rack: '.self::q($n['rack']);
					}
					$parts[] = 'x: '.(int)$n['x'];
					$parts[] = 'y: '.(int)$n['y'];
					$y .= '      - {'.implode(', ', $parts)."}\n";
				}
			}
			else {
				$y .= "    nodes: []\n";
			}

			if ($s['links']) {
				$y .= "    links:\n";
				foreach ($s['links'] as $l) {
					$y .= '      - ['.self::q($l[0]).', '.self::q($l[1])."]\n";
				}
			}
			else {
				$y .= "    links: []\n";
			}
		}

		return $y;
	}

	private function saveYamlAtomic(string $yaml_text): array {
		$dir = dirname($this->path);

		if (file_exists($this->path)) {
			if (!is_writable($this->path)) {
				return ['ok' => false, 'error' => 'Sem permissão de escrita: '.$this->path];
			}
		}
		elseif (!is_writable($dir)) {
			return ['ok' => false, 'error' => 'Sem permissão para criar arquivo em: '.$dir];
		}

		$tmp = $dir.'/'.basename($this->path).'.tmp';
		$bak = $dir.'/'.basename($this->path).'.bak.'.date('Ymd-His');

		if (file_exists($this->path) && !@copy($this->path, $bak)) {
			return ['ok' => false, 'error' => 'Falha ao criar backup em: '.$bak];
		}
		if (@file_put_contents($tmp, $yaml_text, LOCK_EX) === false) {
			return ['ok' => false, 'error' => 'Falha ao escrever tmp: '.$tmp];
		}
		@chmod($tmp, 0640);
		if (!@rename($tmp, $this->path)) {
			@unlink($tmp);
			return ['ok' => false, 'error' => 'Falha ao aplicar arquivo (rename).'];
		}

		return ['ok' => true, 'error' => null, 'backup' => (file_exists($bak) ? $bak : null)];
	}

	/** Igual DependenciesEdit::pushToServer, subcomando dependencies-v2. */
	private function pushToServer(): ?array {
		$bin = '/usr/bin/godesk-client';
		if (!is_file($bin) || !is_executable($bin)) {
			return null;
		}

		$output = [];
		$exit_code = 0;
		exec(escapeshellarg($bin).' dependencies-v2 2>&1', $output, $exit_code);

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
		$sites = $file['sites'];
		$raw = $file['raw'];
		if ($file['error'] !== null && !$save && !$import) {
			$error = $file['error'];
		}

		if ($save || $import) {
			$incoming = null;

			if ($import) {
				if (!function_exists('yaml_parse')) {
					$error = 'Extensão PHP yaml não instalada. Instale php-yaml.';
				}
				else {
					$incoming = $this->parseYaml((string)$this->getInput('import_yaml', ''));
					if ($incoming === null) {
						$error = 'Arquivo importado não é um YAML de dependências v2 válido (esperado a chave "sites").';
					}
				}
			}
			else {
				$decoded = json_decode((string)$this->getInput('topology_json', ''), true);
				if (!is_array($decoded)) {
					$error = 'Dados do editor inválidos (JSON).';
				}
				else {
					$incoming = $this->normalizeSites((array)($decoded['sites'] ?? []));
				}
			}

			if ($error === null && $incoming !== null) {
				// mesmo com erro de validação, devolve o que veio pro editor
				// não perder o desenho
				$sites = $incoming;
				$error = $this->validateSites($sites);
			}

			if ($error === null) {
				$missing = $this->missingHosts($sites);
				if ($missing) {
					$warning = 'Hosts não encontrados neste Zabbix (contam como UP na v2): '.implode(', ', $missing);
				}

				$yaml_text = $this->emitYaml($sites);
				$wr = $this->saveYamlAtomic($yaml_text);
				if (!$wr['ok']) {
					$error = $wr['error'];
				}
				else {
					$raw = $yaml_text;
					$backup = $wr['backup'] ?? null;
					$status = $import
						? 'Arquitetura importada e salva ('.count($sites).' localidades).'
						: 'Arquitetura salva com sucesso.';

					$sync = $this->pushToServer();
					if ($sync !== null) {
						if ($sync['ok']) {
							$sync_status = 'Dependências v2 enviadas para o servidor goDesk com sucesso.';
						}
						else {
							$sync_error = 'Falha ao enviar dependências v2 para o servidor goDesk: '.$sync['output'];
						}
					}
				}
			}
		}

		$hosts = \API::Host()->get(['output' => ['host', 'name'], 'sortfield' => 'host']);
		$groups = \API::HostGroup()->get(['output' => ['name'], 'sortfield' => 'name']);

		$this->setResponse(new CControllerResponseData([
			'title' => _('goDesk - Dependências v2 (arquitetura)'),
			'path' => $this->path,
			'status' => $status,
			'error' => $error,
			'warning' => $warning,
			'backup' => $backup,
			'sync_status' => $sync_status,
			'sync_error' => $sync_error,
			'sites' => $sites,
			'raw' => $raw,
			'hosts' => (array)$hosts,
			'groups' => array_column((array)$groups, 'name'),
			'defaults' => ['root' => self::DEFAULT_ROOT, 'down_event' => self::DEFAULT_DOWN_EVENT]
		]));
	}
}
