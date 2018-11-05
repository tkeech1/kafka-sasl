package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

func produce(topic string, messages, startid int) {

	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers":        "kafka0:9093,kafka1:9093,kafka2:9093",
		"security.protocol":        "ssl",
		"ssl.ca.location":          "server.cer.pem",
		"ssl.certificate.location": "client.cer.pem",
		"ssl.key.location":         "client.key.pem",
	})
	if err != nil {
		panic(err)
	}

	defer p.Close()

	// Delivery report handler for produced messages
	go func() {
		counter := 0
		for e := range p.Events() {
			switch ev := e.(type) {
			case *kafka.Message:
				if ev.TopicPartition.Error != nil {
					fmt.Printf("Delivery failed: %v\n", ev.TopicPartition)
				} else {
					//fmt.Printf("Delivered message to %v\n", ev.TopicPartition)
					counter = counter + 1
				}
			default:
				fmt.Printf("ERROR: %v\n", ev)
			}
		}
		fmt.Printf("delivered %d \n", counter)
	}()

	// Produce messages to topic (asynchronously)
	for i := 0; i < messages; i++ {
		//for _, word := range []string{"Welcome", "to", "the", "Confluent", "Kafka", "Golang", "client", "Welcome", "to", "the", "Confluent", "Kafka", "Golang", "client"} {
		messageNumber := strconv.Itoa(startid + i)
		p.Produce(&kafka.Message{
			TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
			Value:          []byte("testMessage " + messageNumber),
		}, nil)
		//fmt.Println("Producing message " + messageNumber)
		// the default value of queue.buffering.max.ms is 100,000 messages so we need to flush when sending more than 100K messages
		if i%99999 == 0 {
			p.Flush(15 * 1000)
		}
	}

	fmt.Println("Stopping worker... ")
	// Wait for message deliveries before shutting down
	p.Flush(15 * 1000)
}

func consume(topic string) {
	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":        "kafka0:9093,kafka1:9093,kafka2:9093",
		"security.protocol":        "ssl",
		"ssl.ca.location":          "server.cer.pem",
		"ssl.certificate.location": "client.cer.pem",
		"ssl.key.location":         "client.key.pem",
		"group.id":                 "myGroup",
		"auto.offset.reset":        "earliest",
	})

	if err != nil {
		panic(err)
	}

	defer c.Close()

	c.SubscribeTopics([]string{topic, "^aRegex.*[Tt]opic"}, nil)

	counter := 0

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
				//fmt.Printf("%% Message on %s: %s\n", e.TopicPartition, string(e.Value))
				//if e.Headers != nil {
				//	fmt.Printf("%% Headers: %v\n", e.Headers)
				//}
				counter = counter + 1
			case kafka.PartitionEOF:
				fmt.Printf("Reached %v\n", e)
				fmt.Printf("  consumed %d \n", counter)
			case kafka.Error:
				fmt.Fprintf(os.Stderr, "%% Error: %v\n", e)
				run = false
			default:
				fmt.Printf("Ignored %v\n", e)
			}
		}
	}

	fmt.Printf("Closing consumer\n")
}

func createTopic(topic string, numParts, replicationFactor int) {
	// Create a new AdminClient.
	// AdminClient can also be instantiated using an existing
	// Producer or Consumer instance, see NewAdminClientFromProducer and
	// NewAdminClientFromConsumer.
	a, err := kafka.NewAdminClient(&kafka.ConfigMap{
		"bootstrap.servers":        "kafka0:9093,kafka1:9093,kafka2:9093",
		"security.protocol":        "ssl",
		"ssl.ca.location":          "server.cer.pem",
		"ssl.certificate.location": "client.cer.pem",
		"ssl.key.location":         "client.key.pem",
	})
	if err != nil {
		fmt.Printf("Failed to create Admin client: %s\n", err)
		os.Exit(1)
	}
	defer a.Close()

	// Contexts are used to abort or limit the amount of time
	// the Admin call blocks waiting for a result.
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
		fmt.Printf("Failed to create topic: %v\n", err)
		os.Exit(1)
	}

	// Print results
	//for _, result := range results {
	//	fmt.Printf("%s\n", result)
	//}
}

func main() {
	producerWorkers := 3
	//consumerWorkers := 10
	messagesPerWorker := 500000

	/*results := make(chan string)
	var wg sync.WaitGroup
	wg.Add(producerWorkers)
	go func() {
		wg.Wait()
		close(results)
	}()*/

	topic := "repTopic3"
	numPartitions := 1
	replicationFactor := 1
	createTopic(topic, numPartitions, replicationFactor)

	for i := 0; i < producerWorkers; i++ {
		go func(i int) {
			//defer wg.Done()
			//results <- produce(messagesPerWorker)
			fmt.Println("starting worker " + strconv.Itoa(i))
			produce(topic, messagesPerWorker, i*messagesPerWorker)
		}(i)
	}

	//d := make(chan string)
	//<-d

	/*for r := range results {
		fmt.Printf("Delivered message \n", r)
	}*/

	consume(topic)

}
