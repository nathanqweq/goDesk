<?php
/**
 * View: godesk.dependencies.triggers
 *
 * Só serializa o resultado — consumida via fetch() pelo godesk.js pra
 * sugerir nomes de evento do host pai na tela de Dependências.
 */

header('Content-Type: application/json');
echo json_encode($data['result'] ?? ['error' => 'sem resultado']);
