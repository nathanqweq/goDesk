<?php

(new CHtmlPage())
	->setTitle($data['title'])
	->show();

echo '<h2>Conteúdo do godesk-config.yaml</h2>';
echo '<pre>';
print_r($data['config']);
echo '</pre>';
