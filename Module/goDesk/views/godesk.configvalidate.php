<?php
/**
 * View: godesk.config.validate
 *
 * Só serializa o resultado — a tela é consumida via fetch() pelo JS do
 * módulo (godesk.js), não é uma página navegável de verdade.
 */

header('Content-Type: application/json');
echo json_encode($data['result'] ?? ['error' => 'sem resultado']);
