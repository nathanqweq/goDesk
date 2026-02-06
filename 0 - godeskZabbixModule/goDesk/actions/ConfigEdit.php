<?php

namespace Modules\GoDesk\Actions;

use CController;
use CControllerResponseData;

class ConfigEdit extends CController {

	private string $config_path = '/etc/zabbix/godesk-config.yaml';

	public function init(): void {
		// Para simplificar e não travar no CSRF agora:
		// (a gente pode reforçar depois com CSRF certinho)
		$this->disableCsrfValidation();
	}

	protected function checkInput(): bool {
		$fields = [
			'yaml_text' => 'string',
			'save' => 'in 0,1'
		];

		return $this->validateInput($fields);
	}

	protected function checkPermissions(): bool {
		// Trava pelo menos pra SUPER ADMIN (recomendado)
		return (isset(\CWebUser::$data['type']) && \CWebUser::$data['type'] == USER_TYPE_SUPER_ADMIN);
	}

	private function readFile(): array {
		if (!file_exists($this->config_path)) {
			return ['ok' => false, 'error' => 'Arquivo não encontrado: '.$this->config_path, 'text' => ''];
		}
		if (!is_readable($this->config_path)) {
			return ['ok' => false, 'error' => 'Sem permissão de leitura: '.$this->config_path, 'text' => ''];
		}

		$text = file_get_contents($this->config_path);
		if ($text === false) {
			return ['ok' => false, 'error' => 'Falha ao ler arquivo: '.$this->config_path, 'text' => ''];
		}

		return ['ok' => true, 'error' => null, 'text' => $text];
	}

	private function validateYaml(string $text): array {
		if (!function_exists('yaml_parse')) {
			return ['ok' => false, 'error' => 'Extensão PHP yaml não instalada (yaml_parse).'];
		}

		$parsed = @yaml_parse($text);
		if ($parsed === false) {
			return ['ok' => false, 'error' => 'YAML inválido (parse falhou).'];
		}

		// opcional: garantir chaves mínimas
		if (!is_array($parsed) || !isset($parsed['default']) || !isset($parsed['clients'])) {
			return ['ok' => false, 'error' => 'YAML válido, mas faltam chaves esperadas: default/clients.'];
		}

		return ['ok' => true, 'error' => null];
	}

	private function writeFileAtomic(string $text): array {
		if (!is_writable($this->config_path)) {
			return ['ok' => false, 'error' => 'Sem permissão de escrita: '.$this->config_path];
		}

		$dir = dirname($this->config_path);
		$tmp = $dir.'/'.basename($this->config_path).'.tmp';
		$bak = $dir.'/'.basename($this->config_path).'.bak.'.date('Ymd-His');

		// backup antes
		if (!@copy($this->config_path, $bak)) {
			return ['ok' => false, 'error' => 'Falha ao criar backup em: '.$bak];
		}

		// escreve em tmp com lock
		$bytes = @file_put_contents($tmp, $text, LOCK_EX);
		if ($bytes === false) {
			return ['ok' => false, 'error' => 'Falha ao escrever arquivo temporário: '.$tmp];
		}

		@chmod($tmp, 0640);

		// troca atômica
		if (!@rename($tmp, $this->config_path)) {
			@unlink($tmp);
			return ['ok' => false, 'error' => 'Falha ao aplicar arquivo (rename).'];
		}

		return ['ok' => true, 'error' => null, 'backup' => $bak];
	}

	protected function doAction(): void {
		$save = ($this->getInput('save', '0') === '1');

		$status = null;
		$error = null;
		$backup = null;

		if ($save) {
			$new_text = $this->getInput('yaml_text', '');

			$val = $this->validateYaml($new_text);
			if (!$val['ok']) {
				$error = $val['error'];
			}
			else {
				$wr = $this->writeFileAtomic($new_text);
				if (!$wr['ok']) {
					$error = $wr['error'];
				}
				else {
					$status = 'Config salva com sucesso.';
					$backup = $wr['backup'] ?? null;
				}
			}

			$current_text = $new_text; // mantém no textarea
		}
		else {
			$r = $this->readFile();
			if (!$r['ok']) {
				$error = $r['error'];
			}
			$current_text = $r['text'] ?? '';
		}

		$this->setResponse(new CControllerResponseData([
			'title' => _('goDesk - Editar config'),
			'path' => $this->config_path,
			'yaml_text' => $current_text,
			'status' => $status,
			'error' => $error,
			'backup' => $backup
		]));
	}
}
