<?php
/**
 * Action: godesk.dependencies.v2.data
 *
 * JSON pro editor visual da dependência v2 (godesk-topology.js):
 *
 *  - op=group&group=<nome>: hosts do grupo da localidade e dos subgrupos
 *    (ex: "POWERPLANT/GERACAO/UHCC - ..." → Router, Switch/Core, ...).
 *    Os hosts direto no grupo raiz são candidatos a host de status.
 *  - op=state&hosts[]=...: problemas ativos de cada host (nome técnico →
 *    nomes dos problemas), pra simular a decisão com o estado real.
 */

namespace Modules\GoDesk\Actions;

use CController;
use CControllerResponseData;

class DependenciesV2Data extends CController {

	public function init(): void {
		$this->disableCsrfValidation();
	}

	protected function checkInput(): bool {
		return $this->validateInput([
			'op' => 'required|in group,state',
			'group' => 'string',
			'hosts' => 'array'
		]);
	}

	protected function checkPermissions(): bool {
		return (isset(\CWebUser::$data['type']) && \CWebUser::$data['type'] == USER_TYPE_SUPER_ADMIN);
	}

	protected function doAction(): void {
		$result = ($this->getInput('op') === 'group')
			? $this->groupHosts(trim((string)$this->getInput('group', '')))
			: $this->hostState((array)$this->getInput('hosts', []));

		$this->setResponse(new CControllerResponseData(['result' => $result]));
	}

	private function groupHosts(string $group): array {
		if ($group === '') {
			return ['error' => 'informe o grupo'];
		}

		$groups = \API::HostGroup()->get([
			'output' => ['groupid', 'name'],
			'search' => ['name' => $group.'/'],
			'startSearch' => true
		]);
		$exact = \API::HostGroup()->get([
			'output' => ['groupid', 'name'],
			'filter' => ['name' => $group]
		]);
		$groups = array_merge((array)$exact, (array)$groups);
		if (!$groups) {
			return ['error' => 'grupo não encontrado: '.$group];
		}

		$by_id = [];
		foreach ($groups as $g) {
			$by_id[$g['groupid']] = $g['name'];
		}

		$hosts = \API::Host()->get([
			'output' => ['host'],
			'groupids' => array_keys($by_id),
			'selectHostGroups' => ['groupid']
		]);

		$root_hosts = [];
		$sub = [];
		foreach ((array)$hosts as $h) {
			foreach ($h['hostgroups'] ?? [] as $hg) {
				$gname = $by_id[$hg['groupid']] ?? null;
				if ($gname === null) {
					continue;
				}
				if ($gname === $group) {
					$root_hosts[] = $h['host'];
				}
				else {
					$sub[substr($gname, strlen($group) + 1)][] = $h['host'];
				}
			}
		}

		$subgroups = [];
		foreach ($sub as $name => $list) {
			$list = array_values(array_unique($list));
			sort($list, SORT_NATURAL | SORT_FLAG_CASE);
			$subgroups[] = ['name' => (string)$name, 'hosts' => $list];
		}

		return ['root_hosts' => array_values(array_unique($root_hosts)), 'subgroups' => $subgroups];
	}

	private function hostState(array $names): array {
		$names = array_values(array_unique(array_filter(array_map('trim', array_map('strval', $names)), 'strlen')));
		if (!$names) {
			return ['problems' => new \stdClass()];
		}

		$hosts = \API::Host()->get(['output' => ['hostid', 'host'], 'filter' => ['host' => $names]]);
		$name_of = [];
		foreach ((array)$hosts as $h) {
			$name_of[$h['hostid']] = $h['host'];
		}
		if (!$name_of) {
			return ['problems' => new \stdClass()];
		}

		$problems = \API::Problem()->get([
			'output' => ['name', 'objectid'],
			'hostids' => array_keys($name_of),
			'source' => 0,
			'object' => 0
		]);
		if (!$problems) {
			return ['problems' => new \stdClass()];
		}

		$triggers = \API::Trigger()->get([
			'output' => ['triggerid'],
			'triggerids' => array_values(array_unique(array_column($problems, 'objectid'))),
			'selectHosts' => ['hostid']
		]);
		$hosts_of = [];
		foreach ((array)$triggers as $t) {
			$hosts_of[$t['triggerid']] = array_column($t['hosts'] ?? [], 'hostid');
		}

		$out = [];
		foreach ($problems as $p) {
			foreach ($hosts_of[$p['objectid']] ?? [] as $hid) {
				if (isset($name_of[$hid])) {
					$out[$name_of[$hid]][] = $p['name'];
				}
			}
		}

		return ['problems' => $out ?: new \stdClass()];
	}
}
