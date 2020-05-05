package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	goavro "github.com/linkedin/goavro"
	//goavro "gopkg.in/linkedin/goavro.v2"
)

// KafkaProducer is a kafka producer
type KafkaProducer interface {
	Close()
	Events() chan kafka.Event
	Produce(*kafka.Message, chan kafka.Event) error
	Flush(int) int
	ProduceChannel() chan *kafka.Message
}

// KafkaConsumer is a kafka consumer
type KafkaConsumer interface {
	Close() error
	Events() chan kafka.Event
	SubscribeTopics([]string, kafka.RebalanceCb) error
	Poll(int) kafka.Event
}

func timeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	log.Printf("%s took %s", name, elapsed)
}

//func produce(message chan []byte, topic string, producerID int) <-chan int {
func produce(message chan []byte, topic string, producerID int) {
	//sent := make(chan int)
	go func() {
		//defer close(sent)
		defer timeTrack(time.Now(), "Producer "+strconv.Itoa(producerID))

		p := createProducer()
		defer p.Close()

		producedCount := 0
		for m := range message {
			p.ProduceChannel() <- &kafka.Message{
				TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
				Value:          m,
			}
			producedCount = producedCount + 1
			//sent <- 1
			if producedCount%100000 == 0 {
				p.Flush(15 * 1000)
			}
		}

		p.Flush(15 * 1000)
		log.Printf("Producer %d sent %d\n", producerID, producedCount)
	}()
	//return sent
}

func createProducer() KafkaProducer {
	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers":        "kafka0:9093,kafka1:9093,kafka2:9093",
		"security.protocol":        "ssl",
		"ssl.ca.location":          "server.cer.pem",
		"ssl.certificate.location": "client.cer.pem",
		"ssl.key.location":         "client.key.pem",
		"queue.buffering.max.ms":   1000,
		"batch.num.messages":       100000,
		//"debug":                    "msg",
	})
	if err != nil {
		panic(err)
	}
	return p
}

func consume(done <-chan bool, topic string, consumerID int) <-chan int {
	count := make(chan int)
	go func() {
		defer close(count)

		c := createConsumer()
		defer c.Close()

		c.SubscribeTopics([]string{topic, "^aRegex.*[Tt]opic"}, nil)

		consumedCount := 0
		for {
			select {
			case <-done:
				log.Printf("Consumer %d recieved %d \n", consumerID, consumedCount)
				return
			default:
				ev := c.Poll(100)
				if ev == nil {
					continue
				}
				//switch e := ev.(type) {
				switch ev.(type) {
				case *kafka.Message:
					if consumedCount == 0 {
						defer timeTrack(time.Now(), "Consumer "+strconv.Itoa(consumerID))
					}
					consumedCount = consumedCount + 1
					count <- 1
					//text, _ := decodeBinary(e.Value)
					//log.Printf(" Got message: %s\n", string(text))
				}
			}
		}
	}()

	return count
}

func createConsumer() KafkaConsumer {
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
	return c
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
		//"queue.buffering.max.ms":   1000,
		//"batch.num.messages":       100000,
		//"debug":                    "msg",
	})
	if err != nil {
		panic(err)
	}

	value, _ := encodeBinary()

	log.Printf("producing")
	for i := 0; i < 150000; i++ {
		err = p.Produce(&kafka.Message{
			TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
			Value:          value,
			Headers:        []kafka.Header{{Key: "myTestHeader", Value: []byte("header values are binary")}},
		}, nil)

		/*e := <-deliveryChan
		m := e.(*kafka.Message)

		if m.TopicPartition.Error != nil {
			fmt.Printf("Delivery failed: %v\n", m.TopicPartition.Error)
		} else {
			//fmt.Printf("Delivered message to topic %s [%d] at offset %v\n",
			//	*m.TopicPartition.Topic, m.TopicPartition.Partition, m.TopicPartition.Offset)
		}*/
	}

	log.Printf("done")

	p.Flush(15 * 1000)

	log.Printf("done flush")

	//close(deliveryChan)
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

// merge multiple channels into a single channel
func merge(done <-chan bool, cs ...<-chan int) <-chan int {
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
	producerWorkers := 1
	consumerWorkers := 1
	// TODO - 10000 is the minimum number of messages, otherwise the consumers hang... not sure why
	totalMessages := 100000
	numPartitions := 1
	replicationFactor := 1
	topic := "xproducers-" + strconv.Itoa(producerWorkers) + "_partitions-" + strconv.Itoa(numPartitions) + "_repFactor-" + strconv.Itoa(replicationFactor)

	//done := make(chan bool)

	createTopic(topic, numPartitions, replicationFactor)

	log.Printf("Topic: %s\n", topic)
	log.Printf("Producers: %d\n", producerWorkers)
	log.Printf("Consumers: %d\n", consumerWorkers)
	log.Printf("Total Messages: %d \n", totalMessages)
	/*producerResult := make([]<-chan int, producerWorkers)
	messages := make(chan []byte, producerWorkers)
	for i := range producerResult {
		producerResult[i] = make(<-chan int)
		//producerResult[i] = produce(messages, topic, i)
		produce(messages, topic, i)
	}

	/*consumerResult := make([]<-chan int, consumerWorkers)
	go func() {
		for i := range consumerResult {
			consumerResult[i] = make(<-chan int, consumerWorkers)
			consumerResult[i] = consume(done, topic, i)
		}
	}()*/

	/*go func() {
		defer close(messages)
		binary, err := encodeBinary()
		if err != nil {
			log.Printf("error creating binary")
			return
		}
		//for i := 0; i < totalMessages; i++ {
		for {
			messages <- binary
		}
	}()*/

	/*in := merge(done, producerResult...)
	producerMessageCount := 0
	// waits until all channels close or close(done)
	for range in {
		producerMessageCount = producerMessageCount + 1
	}
	log.Printf("Produced: %d\n", producerMessageCount)*/

	simpleProducer(topic)
	simpleConsumer(topic)

	/*for i := 0; i < producerWorkers; i++ {
		go func() {
			simpleProducer(topic)
		}()
	}
	for i := 0; i < consumerWorkers; i++ {
		go func() {
			simpleConsumer(topic)
		}()
	}
	<-done
	*/

	/*out := merge(done, consumerResult...)
	consumerMessageCount := 0
	run := true
	for run {
		select {
		case <-out:
			consumerMessageCount = consumerMessageCount + 1
			if producerMessageCount == consumerMessageCount {
				close(done)
				run = false
			}
		// timeout all go routines to avoid hanging
		case <-time.After(60 * time.Second):
			log.Printf("Timed out waiting for results")
			close(done)
			run = false
		}
	}

	if consumerMessageCount == producerMessageCount {
		log.Printf("Consumed: %d \n", consumerMessageCount)
	} else {
		log.Printf("ERROR: produced %d and consumed %d \n", producerMessageCount, consumerMessageCount)
	}*/

}
