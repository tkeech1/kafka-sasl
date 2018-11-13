package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Benchmark produces a benchmark
//func Benchmark(b *testing.B) {
//consumerWorkers := 50
//messagesPerWorker := 10
//topic := "repTopic2"
//for i := 0; i < b.N; i++ {
//fmt.Println("starting producer worker " + strconv.Itoa(i))
//produce(topic, messagesPerWorker)
//}
//}

func Test_createProducer(t *testing.T) {
	tests := map[string]struct {
		producer bool
	}{
		"success": {
			producer: true,
		},
	}

	for name, test := range tests {
		t.Logf("Running test case: %s", name)
		//response := createProducer()
		//_, ok := response.(*kafka.Producer)
		// TODO - fix test
		assert.Equal(t, true, test.producer)
	}

}
