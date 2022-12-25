package nursery

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/sync/errgroup"
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

// ExampleMultiProducerMultiConsumerUsingErrGroup shows how to use errgroup to
// safely create multiple producers and consumers in a way that you are
// guaranteed that they all finish before exiting the function, and they can be
// canceled from the caller using a context. This pattern is very close to the
// nursery idea, but the nursery idea makes some of this implicit by allowing
// channels to close in a normal defer statement and making the Wait part of
// the structure itself. The raw errgroup idea feels more like Go because of
// flow and indentation, though the use of several contexts in one function is
// a bit strange. It feels like the scope is getting crowded.
func ExampleMultiProducerMultiConsumerUsingErrGroup() {
	const numConsumers = 10
	const numProducers = 20
	const valuesPerProducer = 10

	ch := make(chan string)

	// Set up consumers. These exit when the channel is closed.
	gConsumer := new(errgroup.Group) // no context needed, these close when the channel does.
	for i := 0; i < numConsumers; i++ {
		i := i
		gConsumer.Go(func() error {
			for val := range ch {
				log.Printf("Consumer %d received: %s", i, val)
			}
			return nil
		})
	}

	// Set up a group of producers. Note that this is not complete without the
	// bare goroutine below, which waits for it and then closes the channel to
	// signal consumers that no more data is coming.
	gProducer, ctxProducer := errgroup.WithContext(context.Background())
	for i := 0; i < numProducers; i++ {
		i := i
		gProducer.Go(func() error {
			for j := 0; j < valuesPerProducer; j++ {
				select {
				case ch <- fmt.Sprintf("Producer %d: %d", i, j):
				case <-ctxProducer.Done():
					return fmt.Errorf("Producer %d: %w", i, ctxProducer.Err())
				}
			}
			return nil
		})
	}

	// This awkward little dance is to close the channel when the producers are
	// finished so that the consumers will see a finished channel and exit, as
	// well. Canceling the context can force this to happen early, or, given
	// errgroup semantics, any producer or consumer returning an error will
	// trigger this.
	err := gProducer.Wait()
	close(ch)
	if err != nil {
		log.Fatalf("Error in producers: %v", err)
	}

	if err := gConsumer.Wait(); err != nil {
		log.Fatalf("Error in consumers: %v", err)
	}
}

func TestNursery_MultiProducerMultiConsumer(t *testing.T) {
	// This test shows how to easily and safely build a multi-producer,
	// multi-consumer pattern over a single channel, a case that comes up quite
	// often in practice, and one that has good recipes (see example).
	ctx := context.Background()

	const numProducers = 20
	const valuesPerProducer = 50
	const numConsumers = 10

	// Keep track of number of consumed values, to be updated asynchronously.
	var m sync.Mutex
	numConsumed := 0

	ch := make(chan string)

	// Run a nursery with two nurseries inside of it. One for producers, one for consumers.
	Run(ctx, func(ctx context.Context, n *Nursery) {
		// Producers
		n.Go(func() error {
			defer close(ch) // once producers are all finished (or canceled).
			Run(ctx, func(ctx context.Context, n *Nursery) {
				for i := 0; i < numProducers; i++ {
					i := i
					n.Go(func() error {
						for j := 0; j < valuesPerProducer; j++ {
							select {
							case ch <- fmt.Sprintf("Producer %d: %d", i, j):
							case <-ctx.Done(): // use ctx from what Run passes us in the closure.
								return fmt.Errorf("Producer %d canceled: %w", i, ctx.Err())
							}
						}
						return nil
					})
				}
			})
			return nil
		})

		// Consumers. Note that these naturally end when producers are done,
		// even if they are canceled, because the channel will then be closed.
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

func ExampleGoroutinesSpawnOthersInSameNursery() {
	ctx := context.Background()
	// It is entirely possible to use this pattern to do advanced things like
	// having a goroutine spawn another goroutine in the same nursery.
	Run(ctx, func(ctx context.Context, n *Nursery) {
		n.Go(func() error {
			defer log.Print("Parent exiting")
			log.Print("Parent")
			n.Go(func() error {
				log.Print("Dynamically spawned 1")
				return nil
			})
			n.Go(func() error {
				log.Print("Dynamically spawned 2")
				return nil
			})
			return nil
		})
	})
}
