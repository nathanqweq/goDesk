<?php
/**
 * Action: godesk.flows.data
 *
 * JSON pro simulador do editor de fluxos (godesk-flows.js):
 *
 *  - op=recent: últimos problemas (eventid, nome, host) pra escolher um
 *    evento real e testar o fluxo.
 *  - op=event&eventid=N: os campos que o goDesk teria na hora do alerta —
 *    host, host.name, host.group, host.tag.*, event.tag.*, trigger,
 *    severidade... (mesmos nomes do internal/app/flow.go).
 */

namespace Modules\GoDesk\Actions;

use CController;
use CControllerResponseData;

class FlowsData extends CController {

	private const SEVERITIES = ['Not classified', 'Information', 'Warning', 'Average', 'High', 'Disaster'];

	public function init(): void {
		$this->disableCsrfValidation();
	}

	protected function checkInput(): bool {
		return $this->validateInput([
			'op' => 'required|in recent,event',
			'eventid' => 'id'
		]);
	}

	protected function checkPermissions(): bool {
		return ($this->getUserType() == USER_TYPE_SUPER_ADMIN);
	}

	protected function doAction(): void {
		$result = ($this->getInput('op') === 'recent')
			? $this->recent()
			: $this->event((string)$this->getInput('eventid', ''));

		$this->setResponse(new CControllerResponseData(['result' => $result]));
	}

	private function recent(): array {
		$events = \API::Event()->get([
			'output' => ['eventid', 'name', 'clock', 'severity'],
			'source' => 0,
			'object' => 0,
			'value' => 1,
			'selectHosts' => ['host'],
			'sortfield' => ['clock', 'eventid'],
			'sortorder' => 'DESC',
			'limit' => 40
		]);
		$out = [];
		foreach ((array)$events as $e) {
			$out[] = [
				'eventid' => $e['eventid'],
				'name' => $e['name'],
				'host' => implode(', ', array_column($e['hosts'] ?? [], 'host')),
				'clock' => date('d/m H:i', (int)$e['clock'])
			];
		}
		return ['events' => $out];
	}

	private function event(string $eventid): array {
		if ($eventid === '') {
			return ['error' => 'informe o event ID'];
		}
		$events = \API::Event()->get([
			'output' => ['eventid', 'name', 'clock', 'severity', 'value', 'objectid'],
			'eventids' => [$eventid],
			'selectHosts' => ['hostid', 'host', 'name'],
			'selectTags' => ['tag', 'value']
		]);
		if (!$events) {
			return ['error' => 'evento '.$eventid.' não encontrado (ou sem permissão)'];
		}
		$e = $events[0];
		$h = $e['hosts'][0] ?? ['hostid' => '', 'host' => '', 'name' => ''];

		$host_tags = [];
		$groups = [];
		if ($h['hostid'] !== '') {
			$hosts = \API::Host()->get([
				'output' => ['hostid'],
				'hostids' => [$h['hostid']],
				'selectTags' => ['tag', 'value'],
				'selectHostGroups' => ['name']
			]);
			if ($hosts) {
				$host_tags = $hosts[0]['tags'] ?? [];
				$groups = array_column($hosts[0]['hostgroups'] ?? [], 'name');
			}
		}

		$sev = (int)$e['severity'];
		return [
			'event' => [
				'host' => $h['host'],
				'host.name' => $h['name'],
				'host.id' => $h['hostid'],
				'host.group' => $groups,
				'host_tags' => $host_tags,
				'event_tags' => $e['tags'] ?? [],
				'trigger' => $e['name'],
				'trigger.id' => $e['objectid'],
				'severity' => self::SEVERITIES[$sev] ?? (string)$sev,
				'event.id' => $e['eventid'],
				'event.kind' => ((string)$e['value'] === '1') ? 'problem' : 'recovery',
				'clock' => date('Y-m-d\TH:i', (int)$e['clock'])
			]
		];
	}
}
