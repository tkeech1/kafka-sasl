
build-zk:
	docker build -f Dockerfile_zk -t zookeeper .

build-kafka:
	docker build -f Dockerfile_kafka -t kafka .

build-burrow:
	docker build -f Dockerfile_burrow -t burrow .

prune: 
	docker container prune -f 

run-zk: prune	
	docker run -it \
	-v $(PWD)/zoo.cfg:/opt/zookeeper-3.4.13/conf/zoo.cfg \
	-v $(PWD)/jaas.conf:/opt/zookeeper-3.4.13/conf/jaas.conf \
	-v $(PWD)/java.env:/opt/zookeeper-3.4.13/conf/java.env \
	-v $(PWD)/zookeeper_prom.yml:/opt/zookeeper-3.4.13/conf/zookeeper_prom.yml \
	-v /opt/zookeeper_data:/tmp/zookeeper \
	--workdir /opt/zookeeper-3.4.13/ -p 2181:2181 -p 7070:7070 --name zookeeper zookeeper:latest /bin/bash -c "/opt/zookeeper-3.4.13/bin/zkServer.sh start-foreground"

show-zk-acl: prune
	docker run -it \
	--workdir /opt/zookeeper-3.4.13/ --network host --name client zookeeper:latest /bin/bash -c "/opt/zookeeper-3.4.13/bin/zkCli.sh getAcl /brokers"

run-kafka: prune
	docker run -it \
	-v $(PWD)/server.properties:/opt/kafka_2.12-1.1.1/config/server.properties \
	-v $(PWD)/jaas.conf:/opt/kafka_2.12-1.1.1/config/jaas.conf \
	-v $(PWD)/kafka_prom.yml:/opt/kafka_2.12-1.1.1/config/kafka_prom.yml \
	-v /opt/kafka_data:/tmp/kafka-logs \
	-e KAFKA_OPTS="-Djava.security.auth.login.config=/opt/kafka_2.12-1.1.1/config/jaas.conf -javaagent:/opt/jmx_prometheus_javaagent-0.3.0.jar=7071:/opt/kafka_2.12-1.1.1/config/kafka_prom.yml" \
	-e JMX_PORT=9096 \
	--workdir /opt/kafka_2.12-1.1.1/ --network host -p 9092:9092 -p 7071:7071 -p 9096:9096 --name kafka kafka:latest /bin/bash -c "bin/kafka-server-start.sh config/server.properties"

run-kafkaclient: prune
	# KAFKA_OPTS env var is needed to authenticate to Zookeeper
	docker run -it \
	-v $(PWD)/jaas.conf:/opt/kafka_2.12-1.1.1/config/jaas.conf -e KAFKA_OPTS="-Djava.security.auth.login.config=/opt/kafka_2.12-1.1.1/config/jaas.conf" \
	--workdir /opt/kafka_2.12-1.1.1/ --network host --name kclient kafka:latest /bin/bash

run-kafkamanager: prune
	docker run -it \
	-v $(PWD)/jaas.conf:/kafka-manager/jaas.conf \
	-e ZK_HOSTS="localhost:2181" \
	--network host -p 9000:9000 --name kafkamanager kafka:latest /bin/bash -c "/kafka-manager/kafka-manager-1.3.3.21/bin/kafka-manager -Djava.security.auth.login.config=/kafka-manager/jaas.conf"

run-burrow: prune
	docker run -it \
	-v $(PWD)/burrow.toml:/etc/burrow/burrow.toml \
	--network host -p 8000:8000 --name burrow burrow:latest

stop-zoo-navigator:
	docker stop zoonavigatorapi || true && docker stop zoonavigatorweb || true 

run-zoo-navigator-web: run-zoo-navigator-api	
	docker run -it \
	-e WEB_HTTP_PORT=8001 \
	-e API_HOST=localhost \
	-e API_PORT=9001 \
	--network host -p 8001:8001 --name zoonavigatorweb elkozmon/zoonavigator-web:0.5.0

run-zoo-navigator-api: stop-zoo-navigator prune
	docker run -d \
	-e API_HTTP_PORT=9001 \
	--network host -p 9001:9001 --name zoonavigatorapi elkozmon/zoonavigator-api:0.5.0
