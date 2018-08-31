
build-zk:
	docker build -f Dockerfile_zk -t zookeeper .

build-kafka:
	docker build -f Dockerfile_kafka -t kafka .

prune: 
	docker container prune -f 

run-zk: prune	
	docker run -it \
	-v $(PWD)/zoo.cfg:/opt/zookeeper-3.4.13/conf/zoo.cfg \
	-v $(PWD)/jaas.conf:/opt/zookeeper-3.4.13/conf/jaas.conf \
	-v $(PWD)/java.env:/opt/zookeeper-3.4.13/conf/java.env \
	--workdir /opt/zookeeper-3.4.13/ -p 2181:2181 --name zookeeper zookeeper:latest /bin/bash -c "/opt/zookeeper-3.4.13/bin/zkServer.sh start-foreground"

show-zk-acl: prune
	docker run -it \
	-v $(PWD)/jaas.conf:/opt/zookeeper-3.4.13/conf/jaas.conf \
	-v $(PWD)/client-java.env:/opt/zookeeper-3.4.13/conf/java.env \
	--workdir /opt/zookeeper-3.4.13/ --network host --name client zookeeper:latest /bin/bash -c "/opt/zookeeper-3.4.13/bin/zkCli.sh getAcl /brokers"

run-kafka: prune
	docker run -it \
	-v $(PWD)/server.properties:/opt/kafka_2.12-1.1.1/config/server.properties \
	-v $(PWD)/jaas.conf:/opt/kafka_2.12-1.1.1/config/jaas.conf \
	-e KAFKA_OPTS="-Djava.security.auth.login.config=/opt/kafka_2.12-1.1.1/config/jaas.conf" \
	--workdir /opt/kafka_2.12-1.1.1/ --network host -p 9092:9092 --name kafka kafka:latest /bin/bash -c "bin/kafka-server-start.sh config/server.properties"


	
