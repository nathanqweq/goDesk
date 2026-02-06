<?php

namespace Modules\GoDesk\Actions;

use CController;
use CControllerResponseData;

class Hello extends CController {

	public function init(): void {
		// Para o hello world, sem CSRF mesmo (igual tutorial).
		$this->disableCsrfValidation();
	}

	protected function checkInput(): bool {
		return true;
	}

	protected function checkPermissions(): bool {
		// Hello world: libera pra todo mundo.
		// Depois a gente amarra com roles/perfis certinho.
		return true;
	}

	protected function doAction(): void {
		$data = [
			'title' => _('goDesk - Hello World'),
			'message' => _('Hello World from goDesk!')
		];

		$this->setResponse(new CControllerResponseData($data));
	}
}
