03.03.2019 Minimaler Sensor-Treiber von Roman Hoog Antink
14.12.2021 Rewritten in go

Installationsdetails:
    - installiert in /opt/schneeanzeige
    - cron job (crontab -e -u pi) ruft alle 15 Minuten /opt/schneeanzeige/run_sensor.sh auf
        - run_sensor.sh startet /opt/schneeanzeige/sensor
        - config file in /opt/schneeanzeige/sensor.conf (IP des Laser-Sensors)
        - sensor nimmt eine Messung vor und gibt den Messwert mit einem Timestamp aus im Format
            timestamp="1551638702" value="48.9"
        - run_sensor.sh erstellt ein XML file /tmp/snowsensor_readout.xml
	- server disconnect kopiert dieses file 2 minuten spaeter mittels /home/pulferteann/public_html/snowsensor/store.php

        	<?xml version="1.0" encoding="UTF-8"?><snowdata>  <metering timestamp="1551639603" value="48.9"/></snowdata>

    - log file in /var/log/schneeanzeige/snow_sensor.log
    - logrotate config in /etc/logrotate.d/schneeanzeige
    - go source code in /opt/schneeanzeige/snowsensor.go