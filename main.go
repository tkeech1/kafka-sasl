package main

import (
	"context"
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

func produce(message chan []byte, topic string) <-chan int {
	sent := make(chan int)
	go func() {
		defer close(sent)
		defer timeTrack(time.Now(), "produce")

		p := createProducer()
		defer p.Close()

		i := 0
		for m := range message {
			p.ProduceChannel() <- &kafka.Message{
				TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
				Value:          m,
			}
			i = i + 1
			sent <- 1
			// the default value of Kafka queue.buffering.max.ms is 100,000 messages so we need to flush when sending more than 100K messages
			if i%99999 == 0 {
				p.Flush(5000)
			}
		}

		p.Flush(5000)
	}()
	return sent
}

func createProducer() KafkaProducer {
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
	return p
}

func consume(done <-chan bool, topic string) <-chan int {
	count := make(chan int)
	go func() {

		c := createConsumer()
		defer c.Close()
		defer timeTrack(time.Now(), "consume")
		defer close(count)

		c.SubscribeTopics([]string{topic, "^aRegex.*[Tt]opic"}, nil)

		for {
			select {
			case <-done:
				return
			default:
				ev := c.Poll(100)
				if ev == nil {
					continue
				}
				//switch e := ev.(type) {
				switch ev.(type) {
				case *kafka.Message:
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

func main() {
	producerWorkers := 12
	consumerWorkers := 1
	totalMessages := 10000
	numPartitions := 3
	replicationFactor := 1
	topic := "producers-" + strconv.Itoa(producerWorkers) + "_partitions-" + strconv.Itoa(numPartitions) + "_repFactor-" + strconv.Itoa(replicationFactor)

	done := make(chan bool)
	defer close(done)

	createTopic(topic, numPartitions, replicationFactor)

	log.Printf("Topic: %s\n", topic)
	log.Printf("Producers: %d\n", producerWorkers)
	log.Printf("Total Messages %d \n", totalMessages)
	producerResult := make([]<-chan int, producerWorkers)
	messages := make(chan []byte, producerWorkers)
	for i := range producerResult {
		producerResult[i] = make(<-chan int)
		producerResult[i] = produce(messages, topic)
	}

	go func() {
		defer close(messages)
		binary, err := encodeBinary()
		if err != nil {
			log.Printf("error creating binary")
			return
		}
		for i := 0; i < totalMessages; i++ {
			//messages <- "test message"
			messages <- binary
		}
	}()

	in := merge(done, producerResult...)
	producerMessageCount := 0
	for range in {
		producerMessageCount = producerMessageCount + 1
	}

	log.Printf("Consumers: %d\n", consumerWorkers)
	consumerResult := make([]<-chan int, consumerWorkers)
	for i := range consumerResult {
		consumerResult[i] = make(<-chan int)
		consumerResult[i] = consume(done, topic)
	}

	out := merge(done, consumerResult...)
	consumerMessageCount := 0
	run := true
	for run {
		select {
		case <-out:
			consumerMessageCount = consumerMessageCount + 1
			if producerMessageCount == consumerMessageCount {
				done <- true
				run = false
			}
		// timeout all go routines to avoid hanging
		case <-time.After(10 * time.Second):
			log.Printf("Timed out waiting for results")
			done <- true
			run = false
		}
	}

	if consumerMessageCount == producerMessageCount {
		log.Printf("Received: %d \n", consumerMessageCount)
	} else {
		log.Printf("ERROR: produced %d and consumed %d \n", producerMessageCount, consumerMessageCount)
	}

	wait := make(chan bool)
	<-wait

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
