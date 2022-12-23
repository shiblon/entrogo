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
	// This test shows how to easily and safely build a multi-producer,
	// multi-consumer pattern over a single channel, a case that comes up quite
	// often in practice, and one that has good recipes, but so far they are
	// all pretty awkward.
	//
	// The best recipe I have found over the years looks like the following
	// code comment, and the code in this test that uses nurseries is actually
	// easier to reason about.
	//
	// Note that it takes two explicit groups, error management is kind of
	// weird, and there's that long raw `go` statement in there set up to
	// signal that producers are done by closing the channel, necessitating
	// that `Wait` becalled twice: once inside a goroutine, and once outside
	// just to get the error.
	//
	//   ch := make(chan string)
	//   gProducer, ctxProducer := errgroup.WithContext(context.Background())
	//   gConsumer, ctxConsumer := errgroup.WithContext(context.Background())
	//
	//   for i := 0; i < numProducers; i++ {
	//     i := i
	//     gProducer.Go(func() error {
	//       for j := 0; j < valuesPerProducer; j++ {
	//         ch <- fmt.Sprintf("Producer %d: %d", i, j)
	//       }
	//       return nil
	//     })
	//   }
	//
	//   for i := 0; i < numConsumers; i++ {
	//     i := i
	//     gConsumer.Go(func() error {
	//       for val := range ch {
	//         log.Printf(v)
	//       }
	//       return nil
	//     })
	//   }
	//
	//   go func() {
	//     gProducer.Wait()
	//     close(ch)
	//   }()
	//
	//   if err := gProducer.Wait(); err != nil {
	//     log.Fatalf("Error in producers: %v", err)
	//   }
	//
	//   if err := gConsumer.Wait(); err != nil {
	//     log.Fatalf("Error in consumers: %v", err)
	//   }
	//
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
