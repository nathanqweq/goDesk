<?php
/**
 * Painel 🧪 Testar dos fluxos — incluído por godesk.flows.php (edição) e
 * godesk.flows.view.php (somente leitura). Os campos têm os mesmos nomes do
 * resolvedor do goDesk (internal/app/flow.go).
 *
 * $test_visible: o painel começa aberto (página de visualização).
 */
$test_visible = $test_visible ?? false;

echo '<div class="gd-flow-test" id="gd-flow-test"'.($test_visible ? '' : ' style="display:none"').'>';
echo '<div class="gd-small-title">🧪 Testar o fluxo <span class="gd-muted">(não altera nada — mesma decisão do goDesk na hora do alerta)</span></div>';
echo '<div class="gd-row">';
echo '<div class="gd-field"><label>Carregar de um problema recente</label><select id="gd-flow-test-recent"><option value="">— clique para carregar a lista —</option></select></div>';
echo '<div class="gd-field gd-field-tight"><label>ou Event ID</label><input type="text" id="gd-flow-test-eventid" placeholder="123456"></div>';
echo '<div class="gd-field gd-field-tight" style="min-width:auto;"><button class="gd-btn" type="button" data-gd-flow="test-load">📡 Carregar</button></div>';
echo '</div>';
echo '<div class="gd-row">';
echo '<div class="gd-field"><label>host</label><input type="text" data-gd-test="host"></div>';
echo '<div class="gd-field"><label>host.name</label><input type="text" data-gd-test="host.name"></div>';
echo '<div class="gd-field"><label>trigger</label><input type="text" data-gd-test="trigger"></div>';
echo '<div class="gd-field gd-field-tight"><label>severity</label><select data-gd-test="severity">';
foreach (['Not classified', 'Information', 'Warning', 'Average', 'High', 'Disaster'] as $sv) {
	echo '<option value="'.$sv.'"'.($sv === 'High' ? ' selected' : '').'>'.$sv.'</option>';
}
echo '</select></div>';
echo '</div>';
echo '<div class="gd-row">';
echo '<div class="gd-field gd-field-tight"><label>Tipo</label><select data-gd-test="event.kind"><option value="problem">problema</option><option value="recovery">normalização</option></select></div>';
echo '<div class="gd-field gd-field-tight"><label>Chamado já existe?</label><select data-gd-test="ticket.exists"><option value="false">não</option><option value="true">sim</option></select></div>';
echo '<div class="gd-field gd-field-tight"><label>Data/hora do alerta</label><input type="datetime-local" data-gd-test="now"></div>';
echo '<div class="gd-field"><label>rule</label><input type="text" data-gd-test="rule"></div>';
echo '</div>';
echo '<div class="gd-row">';
echo '<div class="gd-field gd-field-tight"><label>Minutos desde o início <span class="gd-muted">(event.age)</span></label><input type="number" min="0" data-gd-test="event.age" value="0"></div>';
echo '<div class="gd-field gd-field-tight"><label>Reenvios <span class="gd-muted">(event.resends)</span></label><input type="number" min="0" data-gd-test="event.resends" value="0"></div>';
echo '<div class="gd-field gd-field-tight"><label>Reconhecido pelo atendimento? <span class="gd-muted">(event.acked_by_user)</span></label><select data-gd-test="event.acked_by_user"><option value="false">não</option><option value="true">sim</option></select></div>';
echo '<div class="gd-field gd-field-tight"><label>Reconhecido (qualquer um)? <span class="gd-muted">(event.acknowledged)</span></label><select data-gd-test="event.acknowledged"><option value="false">não</option><option value="true">sim</option></select></div>';
echo '</div>';
echo '<div class="gd-row">';
echo '<div class="gd-field"><label>Grupos do host <span class="gd-muted">(um por linha)</span></label><textarea rows="3" data-gd-test="host.group"></textarea></div>';
echo '<div class="gd-field"><label>Tags do host <span class="gd-muted">(Tag=Valor, uma por linha — inclui as herdadas do template)</span></label><textarea rows="3" data-gd-test="host_tags"></textarea></div>';
echo '<div class="gd-field"><label>Tags do evento <span class="gd-muted">(Tag=Valor)</span></label><textarea rows="3" data-gd-test="event_tags"></textarea></div>';
echo '<div class="gd-field"><label>Outros campos <span class="gd-muted">(campo=valor)</span></label><textarea rows="3" data-gd-test="extra" placeholder="cliente=UHCC&#10;value=0"></textarea></div>';
echo '</div>';
echo '<div class="gd-flow-test-result" id="gd-flow-test-result"></div>';
echo '</div>';
