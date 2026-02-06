<?php

(new CHtmlPage())
	->setTitle($data['title'])
	->addItem(new CDiv($data['message']))
	->show();
