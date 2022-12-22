package nursery

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestNursery_Basic(t *testing.T) {
	ctx := context.Background()

	// Make a channel large enough to hold all output, so we don't block.
	ch := make(chan int, 2)

	// Run two goroutines in a nursery, each adding one value.
	Run(ctx, func(ctx context.Context, n *Nursery) {
		n.Go(func() error {
			ch <- 1
			return nil
		})
		n.Go(func() error {
			ch <- 2
			return nil
		})
	})
	// Nursery ran! We can close the channel.
	close(ch)

	log.Printf("ran")

	// Check that the expected values are in there.
	var vals []int
	for val := range ch {
		vals = append(vals, val)
	}
	sort.Sort(sort.IntSlice(vals))

	want := []int{1, 2}
	if diff := cmp.Diff(vals, want); diff != "" {
		t.Errorf("Error in Nursery_Basic (-want +got): %s", diff)
	}
}

func TestNursery_MultiProducerMultiConsumer(t *testing.T) {
	ctx := context.Background()

	ch := make(chan string)

	const numProducers = 20
	const valuesPerProducer = 50
	const numConsumers = 10

	var m sync.Mutex
	numConsumed := 0

	// Run a nursery with two nurseries inside of it. One for producers, one for consumers.
	Run(ctx, func(ctx context.Context, n *Nursery) {
		// Producers
		n.Go(func() error {
			defer close(ch) // once producers are all finished.
			Run(ctx, func(ctx context.Context, n *Nursery) {
				for i := 0; i < numProducers; i++ {
					i := i
					n.Go(func() error {
						for j := 0; j < valuesPerProducer; j++ {
							ch <- fmt.Sprintf("Producer %d: %d", i, j)
						}
						return nil
					})
				}
			})
			return nil
		})

		// Consumers
		n.Go(func() error {
			Run(ctx, func(ctx context.Context, n *Nursery) {
				for i := 0; i < numConsumers; i++ {
					n.Go(func() error {
						for v := range ch {
							log.Print(v)
							m.Lock()
							numConsumed++
							m.Unlock()
						}
						return nil
					})
				}
			})
			return nil
		})
	})

	if got, want := numConsumed, numProducers*valuesPerProducer; got != want {
		t.Errorf("Expected %d values consumed, got %d", want, got)
	}

	log.Print("Finished producing and consuming")
}
