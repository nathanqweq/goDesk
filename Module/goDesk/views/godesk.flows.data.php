<?php
/**
 * View: godesk.flows.data
 *
 * Só serializa o resultado — consumida via fetch() pelo godesk-flows.js.
 */

header('Content-Type: application/json');
echo json_encode($data['result'] ?? ['error' => 'sem resultado'], JSON_UNESCAPED_UNICODE | JSON_INVALID_UTF8_SUBSTITUTE);
