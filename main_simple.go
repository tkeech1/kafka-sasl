package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	goavro "github.com/linkedin/goavro"
	//goavro "gopkg.in/linkedin/goavro.v2"
)

func timeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	log.Printf("%s took %s", name, elapsed)
}

func createTopic(topic string, numParts, replicationFactor int) {
	// Contexts are used to abort or limit the amount of time
	// the Admin call blocks waiting for a result.
	a, err := kafka.NewAdminClient(&kafka.ConfigMap{
		"bootstrap.servers":        "kafka0:9093,kafka1:9093,kafka2:9093",
		"security.protocol":        "ssl",
		"ssl.ca.location":          "server.cer.pem",
		"ssl.certificate.location": "client.cer.pem",
		"ssl.key.location":         "client.key.pem",
	})
	if err != nil {
		log.Printf("Failed to create Admin client: %s\n", err)
		os.Exit(1)
	}
	defer a.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create topics on cluster.
	// Set Admin options to wait for the operation to finish (or at most 60s)
	maxDur, err := time.ParseDuration("60s")
	if err != nil {
		panic("ParseDuration(60s)")
	}
	_, err = a.CreateTopics(
		ctx,
		// Multiple topics can be created simultaneously
		// by providing more TopicSpecification structs here.
		[]kafka.TopicSpecification{{
			Topic:             topic,
			NumPartitions:     numParts,
			ReplicationFactor: replicationFactor}},
		// Admin options
		kafka.SetAdminOperationTimeout(maxDur))
	if err != nil {
		log.Printf("Failed to create topic: %v\n", err)
		os.Exit(1)
	}
}

func simpleProducer(topic string) {
	//deliveryChan := make(chan kafka.Event)

	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers":        "kafka0:9093,kafka1:9093,kafka2:9093",
		"security.protocol":        "ssl",
		"ssl.ca.location":          "server.cer.pem",
		"ssl.certificate.location": "client.cer.pem",
		"ssl.key.location":         "client.key.pem",
		"queue.buffering.max.ms":   50,
		"batch.num.messages":       25000,
		//"debug":                        "msg",
	})
	if err != nil {
		panic(err)
	}

	go func() {
		for e := range p.Events() {
			switch ev := e.(type) {
			case *kafka.Message:
				if ev.TopicPartition.Error != nil {
					fmt.Printf("Delivery failed: %v\n", ev.TopicPartition)
				} else {
					//fmt.Printf("Delivered message to %v\n", ev.TopicPartition)
				}
			}
		}
	}()

	value, _ := encodeBinary()

	log.Printf("Sending messages...")
	producedCount := 0
	for j := 0; j < 10; j++ {
		for i := 0; i < 20000; i++ {
			err = p.Produce(&kafka.Message{
				TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
				Value:          value,
				Headers:        []kafka.Header{{Key: "myTestHeader", Value: []byte("header values are binary")}},
			}, nil)
			producedCount = producedCount + 1
		}
		//log.Printf("j is: %d", j)
		//log.Printf("flushing...")
		left := p.Flush(0)
		//log.Printf(" done.\n")
		//length := p.Len()
		//log.Printf("queue depth: %d", length)
		//log.Printf("messages left in queue: %d", left)
		for left > 0 {
			//log.Printf("flushing...")
			left = p.Flush(0)
			//log.Printf(" done.\n")
			//length := p.Len()
			//log.Printf("queue depth: %d", length)
			//log.Printf("messages left in queue: %d", left)
		}

	}
	log.Printf("Sent %d messages...", producedCount)
}

func simpleConsumer(topic string) {

	defer timeTrack(time.Now(), "Consumer ")

	messageCount := 0

	c, _ := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":        "kafka0:9093,kafka1:9093,kafka2:9093",
		"security.protocol":        "ssl",
		"ssl.ca.location":          "server.cer.pem",
		"ssl.certificate.location": "client.cer.pem",
		"ssl.key.location":         "client.key.pem",
		"group.id":                 "myGroup",
		"session.timeout.ms":       6000,
		"default.topic.config":     kafka.ConfigMap{"auto.offset.reset": "earliest"},
	})
	defer c.Close()

	err := c.SubscribeTopics([]string{topic, "^aRegex.*[Tt]opic"}, nil)
	if err != nil {
		log.Printf("error subscribing to topic")
		return
	}

	run := true
	for run == true {
		select {
		default:
			ev := c.Poll(100)
			if ev == nil {
				continue
			}
			switch e := ev.(type) {
			case *kafka.Message:
				//fmt.Printf("%% Message on %s:\n%s\n",
				//	e.TopicPartition, string(e.Value))
				//if e.Headers != nil {
				//	fmt.Printf("%% Headers: %v\n", e.Headers)
				//}
				messageCount = messageCount + 1
			case kafka.PartitionEOF:
				fmt.Printf("%% Reached %v\n", e)
			case kafka.Error:
				fmt.Fprintf(os.Stderr, "%% Error: %v\n", e)
				run = false
			default:
				log.Printf("Consumed: %d\n", messageCount)
				fmt.Printf("Ignored %v\n", e)
			}
		}
	}

}

func getCodec() (*goavro.Codec, error) {
	codec, err := goavro.NewCodec(`
	{
		"type": "record",
		"name": "LongList",
		"fields" : [
	{"name": "next", "type": ["null", "LongList", {"type": "long", "logicalType": "timestamp-millis"}], "default": null}
		]
	  }`)
	if err != nil {
		log.Printf(" error creating codec ")
		return nil, err
	}
	return codec, nil
}

func encodeBinary() ([]byte, error) {

	codec, err := getCodec()
	if err != nil {
		log.Printf(" error creating codec ")
		return nil, err
	}

	// NOTE: May omit fields when using default value
	textual := []byte(`{"next":{"LongList":{}}}`)

	// Convert textual Avro data (in Avro JSON format) to native Go form
	native, _, err := codec.NativeFromTextual(textual)
	if err != nil {
		log.Printf(" error encoding to native ")
		return nil, err
	}

	// Convert native Go form to binary Avro data
	binary, err := codec.BinaryFromNative(nil, native)
	if err != nil {
		log.Printf(" error encoding to binary ")
		return nil, err
	}

	return binary, err
}

func decodeBinary(binary []byte) ([]byte, error) {

	codec, err := getCodec()
	if err != nil {
		log.Printf(" error creating codec ")
		return nil, err
	}

	// Convert binary Avro data back to native Go form
	native, _, err := codec.NativeFromBinary(binary)
	if err != nil {
		log.Printf(" error decoding to native ")
		return nil, err
	}

	// Convert native Go form to textual Avro data
	textual, err := codec.TextualFromNative(nil, native)
	if err != nil {
		log.Printf(" error decoding from native to text ")
		return nil, err
	}

	return textual, err
}

func main() {
	numPartitions := 3
	replicationFactor := 3
	topic := "simple"

	createTopic(topic, numPartitions, replicationFactor)

	log.Printf("Topic: %s\n", topic)

	simpleProducer(topic)
	simpleConsumer(topic)

}
