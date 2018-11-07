package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

func produce(topic string, messages int) {
	go func() {
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
			//fmt.Printf("producer delivered %d \n", counter)
		}()

		// Produce messages to topic (asynchronously)
		for i := 0; i < messages; i++ {
			p.Produce(&kafka.Message{
				TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
				Value:          []byte("testMessage " + strconv.Itoa(i)),
			}, nil)
			// fmt.Println("Producing message " + messageNumber)
			// the default value of queue.buffering.max.ms is 100,000 messages so we need to flush when sending more than 100K messages
			if i%99999 == 0 {
				p.Flush(15 * 1000)
			}
		}

		//fmt.Println("Stopping producer worker... ")
		// Wait for message deliveries before shutting down
		p.Flush(15 * 1000)
	}()
}

func consume(done <-chan struct{}, topic string) <-chan int {
	count := make(chan int)
	go func() {
		defer close(count)

		c, err := kafka.NewConsumer(&kafka.ConfigMap{
			"bootstrap.servers":        "kafka0:9093,kafka1:9093,kafka2:9093",
			"security.protocol":        "ssl",
			"ssl.ca.location":          "server.cer.pem",
			"ssl.certificate.location": "client.cer.pem",
			"ssl.key.location":         "client.key.pem",
			"group.id":                 "myGroup",
			"auto.offset.reset":        "earliest",
		})

		//fmt.Println("starting consumer worker... ")

		if err != nil {
			panic(err)
		}

		defer c.Close()

		c.SubscribeTopics([]string{topic, "^aRegex.*[Tt]opic"}, nil)

		for {
			select {
			case <-done:
				//fmt.Printf("Closing consumer\n")
				break
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
					//fmt.Println("consumer got message ... ")
					count <- 1
				case kafka.PartitionEOF:
					fmt.Printf("Reached %v\n", e)
				case kafka.Error:
					fmt.Fprintf(os.Stderr, "%% Error: %v\n", e)
				default:
					fmt.Printf("Ignored %v\n", e)
				}
			}
		}
	}()

	return count
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

}

func main() {
	producerWorkers := 50
	consumerWorkers := 50
	messagesPerWorker := 10000
	topic := "repTopic2"
	numPartitions := 9
	replicationFactor := 3

	done := make(chan struct{})
	defer close(done)

	createTopic(topic, numPartitions, replicationFactor)

	fmt.Printf("starting %d producer(s) \n", producerWorkers)
	fmt.Printf("sending %d total message(s) \n", messagesPerWorker*producerWorkers)

	for i := 0; i < producerWorkers; i++ {
		//fmt.Println("starting producer worker " + strconv.Itoa(i))
		produce(topic, messagesPerWorker)
	}

	fmt.Printf("starting %d consumer(s) \n", consumerWorkers)
	consumer := make([]<-chan int, consumerWorkers)
	for i := range consumer {
		consumer[i] = make(<-chan int)
		consumer[i] = consume(done, topic)
	}

	out := merge(done, consumer...)
	messageCount := 0
	for range out {
		messageCount = messageCount + 1
		if messageCount == messagesPerWorker*producerWorkers {
			break
		}
	}

	fmt.Printf("received %d messages\n", messageCount)
}

// merge multiple channels into a single channel
func merge(done <-chan struct{}, cs ...<-chan int) <-chan int {
	var wg sync.WaitGroup
	out := make(chan int)

	// Start an output goroutine for each input channel in cs.  output
	// copies values from c to out until c is closed or it receives a value
	// from done, then output calls wg.Done.
	output := func(c <-chan int) {
		defer wg.Done()
		for n := range c {
			select {
			case out <- n:
			case <-done:
				return
			}
		}
	}
	wg.Add(len(cs))
	for _, c := range cs {
		go output(c)
	}

	// Start a goroutine to close out once all the output goroutines are
	// done.  This must start after the wg.Add call.
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}
