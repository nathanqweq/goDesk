<?php
/**
 * Action: godesk.flows.history
 *
 * Histórico das execuções de fluxo: o que cada alerta fez no fluxo
 * (caminho, campos definidos, escalonamento, HTTP, erros). Os dados ficam
 * no servidor goDesk (godesk-flows-history.jsonl) e chegam aqui pelo
 * godesk-client flows-history → GET /flows/history.
 *
 * Só Super admin: o histórico vem do servidor e mostra eventos de todos os
 * hosts, sem passar pelas permissões do Zabbix.
 */

namespace Modules\GoDesk\Actions;

use CController;
use CControllerResponseData;

class FlowsHistory extends CController {

	private string $client_bin = '/usr/bin/godesk-client';

	public function init(): void {
		$this->disableCsrfValidation();
	}

	protected function checkInput(): bool {
		return $this->validateInput([
			'flow' => 'string',
			'rule' => 'string',
			'event' => 'string',
			'q' => 'string',
			'limit' => 'int32'
		]);
	}

	protected function checkPermissions(): bool {
		return ($this->getUserType() == USER_TYPE_SUPER_ADMIN);
	}

	protected function doAction(): void {
		$filters = [
			'flow' => trim((string)$this->getInput('flow', '')),
			'rule' => trim((string)$this->getInput('rule', '')),
			'event' => trim((string)$this->getInput('event', '')),
			'q' => trim((string)$this->getInput('q', '')),
			'limit' => max(10, min(1000, (int)$this->getInput('limit', 200)))
		];

		$entries = [];
		$error = null;
		if (!is_file($this->client_bin) || !is_executable($this->client_bin)) {
			$error = 'godesk-client não encontrado em '.$this->client_bin.'.';
		}
		else {
			$output = [];
			$code = 0;
			exec(escapeshellarg($this->client_bin).' flows-history '.escapeshellarg(http_build_query($filters)).' 2>&1', $output, $code);
			$text = implode("\n", $output);
			$decoded = json_decode($text, true);
			if ($code !== 0 || !is_array($decoded)) {
				$error = 'Falha ao buscar o histórico no servidor goDesk: '.trim($text);
			}
			else {
				$entries = (array)($decoded['entries'] ?? []);
			}
		}

		$this->setResponse(new CControllerResponseData([
			'title' => _('goDesk - Fluxos: histórico'),
			'filters' => $filters,
			'entries' => $entries,
			'error' => $error
		]));
	}
}
