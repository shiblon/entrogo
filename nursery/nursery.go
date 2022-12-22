// Package nursery implements Nurseries, a form of structured concurrency as
// described in
// https://vorpus.org/blog/notes-on-structured-concurrency-or-go-statement-considered-harmful/.
package nursery

import (
	"context"
	"fmt"
	"log"

	"golang.org/x/sync/errgroup"
)

// Nursery provides a structured way to work with parent and child goroutine
// lifecycles.
type Nursery struct {
	g *errgroup.Group
}

// Block is a function that is executed in the context of a Nursery, which can
// be used to run multiple goroutines that all must exit before returning
// control to the caller of nursery.Run.
type Block func(context.Context, *Nursery)

// Run creates a nursery that runs the given function. Run executes the block,
// running any requested goroutines until they are all completed, using the
// same semantics as an ErrGroup with a Context.
func Run(ctx context.Context, block Block) error {
	g, childCtx := errgroup.WithContext(ctx)
	n := &Nursery{
		g: g,
	}

	block(childCtx, n)

	log.Printf("Getting there")

	if err := g.Wait(); err != nil {
		return fmt.Errorf("Run nursery: %w", err)
	}

	return nil
}

// Go spawns a goroutine for the given function, ensuring that it will be waited on.
// The function is expected to accept a context and properly deal with context
// cancellation.
func (n *Nursery) Go(f func() error) {
	n.g.Go(f)
}
