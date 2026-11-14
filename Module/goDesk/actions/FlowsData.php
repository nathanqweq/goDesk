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

	// também usado pela visualização somente leitura (godesk.flows.view):
	// qualquer usuário logado, com as permissões de host dele no Zabbix
	protected function checkPermissions(): bool {
		return ($this->getUserType() >= USER_TYPE_ZABBIX_USER);
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
		$extra = [];
		if ($h['hostid'] !== '') {
			$params = [
				'output' => ['hostid'],
				'hostids' => [$h['hostid']],
				'selectTags' => ['tag', 'value'],
				'selectHostGroups' => ['name'],
				'selectInventory' => 'extend',
				'selectMacros' => ['macro', 'value'],
				// tags herdadas dos templates (o goDesk também considera)
				'selectInheritedTags' => ['tag', 'value']
			];
			try {
				$hosts = \API::Host()->get($params);
			}
			catch (\Throwable $ex) {
				$hosts = false;
			}
			if ($hosts === false) {
				unset($params['selectInheritedTags']);
				$hosts = \API::Host()->get($params);
			}
			if ($hosts) {
				$seen = [];
				foreach (array_merge($hosts[0]['tags'] ?? [], $hosts[0]['inheritedTags'] ?? []) as $t) {
					$k = mb_strtolower($t['tag'])."\0".$t['value'];
					if (!isset($seen[$k])) {
						$seen[$k] = true;
						$host_tags[] = ['tag' => $t['tag'], 'value' => $t['value']];
					}
				}
				$groups = array_column($hosts[0]['hostgroups'] ?? [], 'name');
				// campos extras do fluxo (inventário, macros do host)
				if (is_array($hosts[0]['inventory'] ?? null)) {
					foreach ($hosts[0]['inventory'] as $k => $v) {
						if (is_string($v) && $v !== '' && $k !== 'hostid' && $k !== 'inventory_mode') {
							$extra[] = 'host.inventory.'.$k.'='.$v;
						}
					}
				}
				foreach ($hosts[0]['macros'] ?? [] as $m) {
					$name = trim((string)$m['macro'], '{}$');
					$extra[] = 'host.macro.'.$name.'='.(string)($m['value'] ?? '');
				}
			}
			$extra[] = 'host.problems='.(int)\API::Problem()->get(['countOutput' => true, 'hostids' => [$h['hostid']], 'source' => 0, 'object' => 0]);
		}

		// reconhecido pelo atendimento: ack de alguém além do usuário do
		// goDesk (identificado pelas mensagens "Chamado ..." que ele grava)
		$acks = \API::Event()->get([
			'output' => ['eventid', 'acknowledged'],
			'eventids' => [$eventid],
			'select_acknowledges' => ['userid', 'action', 'message']
		]);
		$acked = $acked_user = false;
		if ($acks) {
			$acked = ((string)$acks[0]['acknowledged'] === '1');
			$godesk = [];
			foreach ($acks[0]['acknowledges'] ?? [] as $a) {
				$m = mb_strtolower(trim((string)$a['message']));
				foreach (['chamado ', 'normalização recebida', 'normalizacao recebida', 'godesk'] as $p) {
					if (strpos($m, $p) === 0) {
						$godesk[$a['userid']] = true;
					}
				}
			}
			foreach ($acks[0]['acknowledges'] ?? [] as $a) {
				if (((int)$a['action'] & 2) && !isset($godesk[$a['userid']])) {
					$acked_user = $acked;
					break;
				}
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
				'event.age' => (string)max(0, (int)floor((time() - (int)$e['clock']) / 60)),
				'event.acknowledged' => $acked ? 'true' : 'false',
				'event.acked_by_user' => $acked_user ? 'true' : 'false',
				'extra' => $extra,
				'clock' => date('Y-m-d\TH:i', (int)$e['clock'])
			]
		];
	}
}
