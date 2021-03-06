#ZOOKEEPER_VERSION=zookeeper-3.5.4-beta
ZOOKEEPER_VERSION=zookeeper-3.4.13
ZOOKEEPER_IMAGE=kafka:latest
KAFKA_VERSION=kafka_2.12-1.1.1
KAFKA_IMAGE=kafka:latest

build-kafka-zk:
	docker build --no-cache -f Dockerfile_kafka_zk -t kafka .
	# copy the cert files out of the kafka image so they can be mounted into the go client container
	docker run -v $$PWD:/opt/mount --rm --entrypoint cp kafka /opt/client.cer.pem /opt/client.key.pem /opt/server.cer.pem /opt/mount/
	sudo chmod 755 client.cer.pem client.key.pem server.cer.pem

build-burrow:
	docker build -f Dockerfile_burrow -t burrow .

prune: 
	docker container prune -f 

stop-zk:
	docker stop zoo1 || true && docker rm zoo1 || true
	docker stop zoo2 || true && docker rm zoo2 || true
	docker stop zoo3 || true && docker rm zoo3 || true

create-network:
	docker network create knet

destroy-network:
	docker network rm knet || true

run-zk: prune destroy-network create-network
	docker run -d \
	-v $(PWD)/zoo.cfg:/opt/$(ZOOKEEPER_VERSION)/conf/zoo.cfg \
	-v $(PWD)/log4j.properties_zk:/opt/$(ZOOKEEPER_VERSION)/conf/log4j.properties \
	-v $(PWD)/jaas.conf:/opt/$(ZOOKEEPER_VERSION)/conf/jaas.conf \
	-v $(PWD)/java.env:/opt/$(ZOOKEEPER_VERSION)/conf/java.env \
	-v $(PWD)/zookeeper_prom.yml:/opt/$(ZOOKEEPER_VERSION)/conf/zookeeper_prom.yml \
	-v /opt/zoo1_data:/tmp/zookeeper \
	-v $(PWD)/myid.1:/tmp/zookeeper/myid \
	--workdir /opt/$(ZOOKEEPER_VERSION)/ -p 2181:2181 -p 7071:7070 --name zoo1 \
	--network knet \
	$(ZOOKEEPER_IMAGE) /bin/bash -c "/opt/$(ZOOKEEPER_VERSION)/bin/zkServer.sh start-foreground"
	docker run -d \
	-v $(PWD)/zoo.cfg:/opt/$(ZOOKEEPER_VERSION)/conf/zoo.cfg \
	-v $(PWD)/log4j.properties_zk:/opt/$(ZOOKEEPER_VERSION)/conf/log4j.properties \
	-v $(PWD)/jaas.conf:/opt/$(ZOOKEEPER_VERSION)/conf/jaas.conf \
	-v $(PWD)/java.env:/opt/$(ZOOKEEPER_VERSION)/conf/java.env \
	-v $(PWD)/zookeeper_prom.yml:/opt/$(ZOOKEEPER_VERSION)/conf/zookeeper_prom.yml \
	-v /opt/zoo2_data:/tmp/zookeeper \
	-v $(PWD)/myid.2:/tmp/zookeeper/myid \
	--workdir /opt/$(ZOOKEEPER_VERSION)/ -p 2182:2181 -p 7072:7070 --name zoo2 \
	--network knet \
	$(ZOOKEEPER_IMAGE) /bin/bash -c "/opt/$(ZOOKEEPER_VERSION)/bin/zkServer.sh start-foreground"
	docker run -d \
	-v $(PWD)/zoo.cfg:/opt/$(ZOOKEEPER_VERSION)/conf/zoo.cfg \
	-v $(PWD)/log4j.properties_zk:/opt/$(ZOOKEEPER_VERSION)/conf/log4j.properties \
	-v $(PWD)/jaas.conf:/opt/$(ZOOKEEPER_VERSION)/conf/jaas.conf \
	-v $(PWD)/java.env:/opt/$(ZOOKEEPER_VERSION)/conf/java.env \
	-v $(PWD)/zookeeper_prom.yml:/opt/$(ZOOKEEPER_VERSION)/conf/zookeeper_prom.yml \
	-v /opt/zoo3_data:/tmp/zookeeper \
	-v $(PWD)/myid.3:/tmp/zookeeper/myid \
	--workdir /opt/$(ZOOKEEPER_VERSION)/ -p 2183:2181 -p 7073:7070 --name zoo3 \
	--network knet \
	$(ZOOKEEPER_IMAGE) /bin/bash -c "/opt/$(ZOOKEEPER_VERSION)/bin/zkServer.sh start-foreground"
	until echo ruok | nc localhost 2181 | grep -q imok; do echo "Waiting for zookeeper to be ready..."; sleep 1; done
	until echo ruok | nc localhost 2182 | grep -q imok; do echo "Waiting for zookeeper to be ready..."; sleep 1; done
	until echo ruok | nc localhost 2183 | grep -q imok; do echo "Waiting for zookeeper to be ready..."; sleep 1; done

show-zk-acl: prune
	docker exec -it \
	--workdir /opt/$(ZOOKEEPER_VERSION)/ zoo3 /bin/bash -c "/opt/$(ZOOKEEPER_VERSION)/bin/zkCli.sh getAcl /brokers"

stop-kafka:
	docker stop kafka1 || true && docker rm kafka1 || true
	docker stop kafka2 || true && docker rm kafka2 || true
	docker stop kafka0 || true && docker rm kafka0 || true

run-kafka: prune
	docker run -d \
	-v $(PWD)/server.0.properties:/opt/$(KAFKA_VERSION)/config/server.properties \
	-v $(PWD)/jaas.conf:/opt/$(KAFKA_VERSION)/config/jaas.conf \
	-v $(PWD)/kafka_prom.yml:/opt/$(KAFKA_VERSION)/config/kafka_prom.yml \
	-v $(PWD)/client-ssl.properties:/opt/$(KAFKA_VERSION)/config/client-ssl.properties \
	-v /opt/kafka0_data:/tmp/kafka-logs \
	-e KAFKA_OPTS="-Djava.security.auth.login.config=/opt/$(KAFKA_VERSION)/config/jaas.conf -javaagent:/opt/jmx_prometheus_javaagent-0.3.0.jar=7071:/opt/$(KAFKA_VERSION)/config/kafka_prom.yml" \
	-e JMX_PORT=9096 \
	--workdir /opt/$(KAFKA_VERSION)/ --network knet -p 9092:9092 -p 7074:7071 -p 9096:9096 --name kafka0 $(KAFKA_IMAGE) /bin/bash -c "bin/kafka-server-start.sh config/server.properties"
	docker run -d \
	-v $(PWD)/server.1.properties:/opt/$(KAFKA_VERSION)/config/server.properties \
	-v $(PWD)/jaas.conf:/opt/$(KAFKA_VERSION)/config/jaas.conf \
	-v $(PWD)/kafka_prom.yml:/opt/$(KAFKA_VERSION)/config/kafka_prom.yml \
	-v $(PWD)/client-ssl.properties:/opt/$(KAFKA_VERSION)/config/client-ssl.properties \
	-v /opt/kafka1_data:/tmp/kafka-logs \
	-e KAFKA_OPTS="-Djava.security.auth.login.config=/opt/$(KAFKA_VERSION)/config/jaas.conf -javaagent:/opt/jmx_prometheus_javaagent-0.3.0.jar=7071:/opt/$(KAFKA_VERSION)/config/kafka_prom.yml" \
	-e JMX_PORT=9096 \
	--workdir /opt/$(KAFKA_VERSION)/ --network knet -p 9093:9092 -p 7075:7071 -p 9097:9096 --name kafka1 $(KAFKA_IMAGE) /bin/bash -c "bin/kafka-server-start.sh config/server.properties"	
	docker run -d \
	-v $(PWD)/server.2.properties:/opt/$(KAFKA_VERSION)/config/server.properties \
	-v $(PWD)/jaas.conf:/opt/$(KAFKA_VERSION)/config/jaas.conf \
	-v $(PWD)/kafka_prom.yml:/opt/$(KAFKA_VERSION)/config/kafka_prom.yml \
	-v $(PWD)/client-ssl.properties:/opt/$(KAFKA_VERSION)/config/client-ssl.properties \
	-v /opt/kafka2_data:/tmp/kafka-logs \
	-e KAFKA_OPTS="-Djava.security.auth.login.config=/opt/$(KAFKA_VERSION)/config/jaas.conf -javaagent:/opt/jmx_prometheus_javaagent-0.3.0.jar=7071:/opt/$(KAFKA_VERSION)/config/kafka_prom.yml" \
	-e JMX_PORT=9096 \
	--workdir /opt/$(KAFKA_VERSION)/ --network knet -p 9094:9092 -p 7076:7071 -p 9098:9096 --name kafka2 $(KAFKA_IMAGE) /bin/bash -c "bin/kafka-server-start.sh config/server.properties"

run-kafkaclient: prune
	# KAFKA_OPTS env var is needed to authenticate to Zookeeper
	docker run -it \
	-v $(PWD)/jaas.conf:/opt/$(KAFKA_VERSION)/config/jaas.conf \
	-v $(PWD)/client-ssl.properties:/opt/$(KAFKA_VERSION)/config/client-ssl.properties \
	-e KAFKA_OPTS="-Djava.security.auth.login.config=/opt/$(KAFKA_VERSION)/config/jaas.conf" \
	-e CLIENT_JVMFLAGS="-Dzookeeper.clientCnxnSocket=org.apache.zookeeper.ClientCnxnSocketNetty -Dzookeeper.client.secure=true -Dzookeeper.ssl.keyStore.location=/opt/server.keystore.jks -Dzookeeper.ssl.keyStore.password=keystorepassword -Dzookeeper.ssl.trustStore.location=/opt/server.truststore.jks -Dzookeeper.ssl.trustStore.password=keystorekeypassword" \
	--workdir /opt/$(KAFKA_VERSION)/ --network knet --name kclient $(KAFKA_IMAGE) /bin/bash
	# to test producer with SSL
	#./bin/kafka-console-producer.sh --broker-list kafka0:9093 --topic test --producer.config config/client-ssl.properties
	# to test consumer with SSL
	#./bin/kafka-console-consumer.sh --bootstrap-server kafka1:9093 --topic producers-3_partitions-3_repFactor-3 --consumer.config config/client-ssl.properties --from-beginning

stop-kafkaclient: 
	docker stop client || true && docker rm client || true

run-kafkamanager: prune
	docker run -d \
	-v $(PWD)/jaas.conf:/kafka-manager/jaas.conf \
	-e ZK_HOSTS="zoo1:2181,zoo2:2181,zoo3:2181" \
	--network knet -p 9000:9000 --name kafkamanager $(KAFKA_IMAGE) /bin/bash -c "/kafka-manager/kafka-manager-1.3.3.21/bin/kafka-manager -Djava.security.auth.login.config=/kafka-manager/jaas.conf"

stop-kafkamanager: 
	docker stop kafkamanager || true && docker rm kafkamanager || true

run-burrow: prune
	docker run -d \
	-v $(PWD)/burrow.toml:/etc/burrow/burrow.toml \
	-v $(PWD)/client.cer.pem:/etc/burrow/client.cer.pem \
	-v $(PWD)/client.key.pem:/etc/burrow/client.key.pem \
	-v $(PWD)/server.cer.pem:/etc/burrow/server.cer.pem \
	--network knet -p 8000:8000 --name burrow burrow:latest
	
stop-burrow: 
	docker stop burrow || true && docker rm burrow || true

stop-zoo-navigator:
	docker stop zoonavigatorweb || true && docker rm zoonavigatorweb || true
	docker stop zoonavigatorapi || true && docker rm zoonavigatorapi || true

run-zoo-navigator-web: run-zoo-navigator-api	
	docker run -d \
	-e WEB_HTTP_PORT=8001 \
	-e API_HOST=zoonavigatorapi \
	-e API_PORT=9001 \
	--network knet -p 8001:8001 --name zoonavigatorweb elkozmon/zoonavigator-web:0.5.0

run-zoo-navigator-api: stop-zoo-navigator prune
	docker run -d \
	-e API_HTTP_PORT=9001 \
	--network knet -p 9001:9001 --name zoonavigatorapi elkozmon/zoonavigator-api:0.5.0

run-all: run-zk run-kafka 
	#run-kafkamanager run-burrow run-zoo-navigator-web

stop-all: stop-kafka stop-zk stop-kafkaclient stop-kafkamanager stop-burrow stop-zoo-navigator prune destroy-network clean

clean: 
	sudo rm -rf /opt/zoo1_data/* 
	sudo rm -rf /opt/zoo2_data/* 
	sudo rm -rf /opt/zoo3_data/* 
	sudo rm -rf /opt/kafka0_data/* 
	sudo rm -rf /opt/kafka1_data/* 
	sudo rm -rf /opt/kafka2_data/* 

go-client-image:
	docker build -f Dockerfile_go -t kafkaload .

go-client-run: go-client-image
	docker run -it --rm --network knet \
	-v $(PWD):/root/kafka-sasl/ \
	kafkaload go run main_simple.go

go-client-tidy: go-client-image
	docker run -it --rm --network knet \
	-v $(PWD):/root/kafka-sasl/ \
	kafkaload go mod tidy

go-client-test: go-client-image
	docker run -it --rm --network knet \
	-v $(PWD):/root/kafka-sasl/ \
	kafkaload /bin/bash -c "go test"