<?php
/**
 * View: godesk.topology.data
 *
 * Só serializa o resultado — consumida via fetch() pelo godesk-topology.js.
 */

header('Content-Type: application/json');
echo json_encode($data['result'] ?? ['error' => 'sem resultado'], JSON_UNESCAPED_UNICODE);
