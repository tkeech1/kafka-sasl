package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

// KafkaProducer is a kafka producer
type KafkaProducer interface {
	Close()
	Events() chan kafka.Event
	Produce(*kafka.Message, chan kafka.Event) error
	Flush(int) int
}

type Done struct{}

// KafkaConsumer is a kafka consumer
type KafkaConsumer interface {
	Close() error
	SubscribeTopics([]string, kafka.RebalanceCb) error
	Poll(int) kafka.Event
}

func timeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	log.Printf("%s took %s", name, elapsed)
}

func produce(message chan string, topic string) <-chan int {
	count := make(chan int)
	go func() {
		defer close(count)
		defer timeTrack(time.Now(), "deliveryReport")

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
			for e := range p.Events() {
				switch ev := e.(type) {
				case *kafka.Message:
					if ev.TopicPartition.Error != nil {
						log.Printf("Delivery failed: %v\n", ev.TopicPartition)
					} else {
						//log.Printf("Delivered message to %v\n", ev.TopicPartition)
						count <- 1
					}
				default:
					log.Printf("ERROR: %v\n", ev)
				}
			}
		}()

		i := 0
		// Produce messages to topic (asynchronously)
		for m := range message {
			//log.Printf("Producing message %d: %s", i, m)
			p.Produce(&kafka.Message{
				TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
				Value:          []byte(m + strconv.Itoa(i)),
			}, nil)
			i = i + 1
			// the default value of queue.buffering.max.ms is 100,000 messages so we need to flush when sending more than 100K messages
			if i%99999 == 0 {
				p.Flush(15 * 1000)
			}
		}

		//fmt.Println("Stopping producer worker... ")
		// Wait for message deliveries before shutting down
		p.Flush(15 * 1000)

	}()
	return count
}

func consume(done <-chan struct{}, topic string) <-chan int {
	count := make(chan int)
	go func() {
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
				close(count)
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
					log.Printf("Reached %v\n", e)
				case kafka.Error:
					log.Printf("Error: %v\n", e)
				default:
					//log.Printf("Ignored %v\n", e)
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
		log.Printf("Failed to create Admin client: %s\n", err)
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
		log.Printf("Failed to create topic: %v\n", err)
		os.Exit(1)
	}

}

func main() {
	producerWorkers := 5
	consumerWorkers := 10
	totalMessages := 10000
	topic := "repTopic"
	numPartitions := 9
	replicationFactor := 3

	done := make(chan struct{})
	defer close(done)

	createTopic(topic, numPartitions, replicationFactor)

	log.Printf("starting %d producer(s) \n", producerWorkers)
	log.Printf("sending %d total message(s) \n", totalMessages)
	producer := make([]<-chan int, producerWorkers)
	messages := make(chan string, producerWorkers)
	for i := range producer {
		producer[i] = make(<-chan int)
		producer[i] = produce(messages, topic)
	}

	go func() {
		for i := 0; i < totalMessages; i++ {
			messages <- "test message"
		}
		close(messages)
	}()

	log.Printf("starting %d consumer(s) \n", consumerWorkers)
	consumer := make([]<-chan int, consumerWorkers)
	for i := range consumer {
		consumer[i] = make(<-chan int)
		consumer[i] = consume(done, topic)
	}

	log.Printf("processing delivered messages")
	in := merge(done, producer...)
	producerCount := 0
	for range in {
		producerCount = producerCount + 1
		if producerCount == totalMessages {
			break
		}
	}
	log.Printf("delivered %d messages\n", producerCount)

	// need to be able to count consumer messages without closing the consumer channels
	out := merge(done, consumer...)
	consumerCount := 0
	for range out {
		consumerCount = consumerCount + 1
		if producerCount == consumerCount {
			done <- Done{}
			break
		}
	}

	if consumerCount == producerCount {
		log.Printf("received %d messages\n", consumerCount)
	} else {
		log.Printf("ERROR: produced %d and consumed %d \n", producerCount, consumerCount)
	}

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
	//go func() {
	//	wg.Wait()
	//	close(out)
	//}()
	return out
}
