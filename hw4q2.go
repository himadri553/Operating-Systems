package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
)

/***************
 * Utilities
 ***************/
type Counter struct {
	EnqOK    uint64
	DeqOK    uint64
	DeqEmpty uint64
}

func (c *Counter) add(other Counter) {
	atomic.AddUint64(&c.EnqOK, other.EnqOK)
	atomic.AddUint64(&c.DeqOK, other.DeqOK)
	atomic.AddUint64(&c.DeqEmpty, other.DeqEmpty)
}

func busyWork(nanos int) {
	if nanos <= 0 {
		return
	}
	start := time.Now()
	x := uint64(1469598103934665603) // simple mix
	for time.Since(start) < time.Duration(nanos)*time.Nanosecond {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
	}
	_ = x
}

/***************
 * Two-lock queue (Figure 29.9 style)
 ***************/
type tlqNode struct {
	val  int
	next *tlqNode
}

type TwoLockQueue struct {
	head      *tlqNode
	tail      *tlqNode
	headMutex sync.Mutex
	tailMutex sync.Mutex
}

func NewTwoLockQueue() *TwoLockQueue {
	dummy := &tlqNode{}
	return &TwoLockQueue{
		head: dummy,
		tail: dummy,
	}
}

func (q *TwoLockQueue) Enqueue(v int) {
	n := &tlqNode{val: v}
	q.tailMutex.Lock()
	q.tail.next = n
	q.tail = n
	q.tailMutex.Unlock()
}

func (q *TwoLockQueue) Dequeue() (int, bool) {
	q.headMutex.Lock()
	h := q.head
	n := h.next
	if n == nil {
		q.headMutex.Unlock()
		return 0, false
	}
	v := n.val
	q.head = n
	q.headMutex.Unlock()
	return v, true
}

/***************
 * Michael & Scott lock-free queue
 * Assumptions:
 *  - GC means we don't recycle nodes -> practical ABA risk is negligible for this lab.
 *  - We still follow the classic MS algorithm with a dummy node.
 ***************/
type lfNode struct {
	val  int
	next atomic.Pointer[lfNode]
}

type MSQueue struct {
	head atomic.Pointer[lfNode]
	tail atomic.Pointer[lfNode]
}

func NewMSQueue() *MSQueue {
	dummy := &lfNode{}
	q := &MSQueue{}
	q.head.Store(dummy)
	q.tail.Store(dummy)
	return q
}

func (q *MSQueue) Enqueue(v int) {
	n := &lfNode{val: v}
	for {
		tail := q.tail.Load()
		next := tail.next.Load()
		if tail == q.tail.Load() { // still consistent
			if next == nil {
				// try link new node
				if tail.next.CompareAndSwap(nil, n) {
					// swing tail
					q.tail.CompareAndSwap(tail, n)
					return
				}
			} else {
				// tail is behind, help advance it
				q.tail.CompareAndSwap(tail, next)
			}
		}
		// retry
		runtime.Gosched()
	}
}

func (q *MSQueue) Dequeue() (int, bool) {
	for {
		head := q.head.Load()
		tail := q.tail.Load()
		next := head.next.Load()
		if head == q.head.Load() {
			if next == nil {
				// empty
				return 0, false
			}
			if head == tail {
				// tail behind, help advance
				q.tail.CompareAndSwap(tail, next)
				continue
			}
			v := next.val
			if q.head.CompareAndSwap(head, next) {
				return v, true
			}
		}
		runtime.Gosched()
	}
}

/***************
 * Benchmark harness
 ***************/
type Queue interface {
	Enqueue(v int)
	Dequeue() (int, bool)
}

func runProducers(ctx context.Context, wg *sync.WaitGroup, q Queue, id int, c *Counter, workNS int) {
	defer wg.Done()
	r := rand.New(rand.NewSource(time.Now().UnixNano() + int64(id)*1337))
	for {
		select {
		case <-ctx.Done():
			return
		default:
			q.Enqueue(int(r.Uint32()))
			atomic.AddUint64(&c.EnqOK, 1)
			busyWork(workNS)
		}
	}
}

func runConsumers(ctx context.Context, wg *sync.WaitGroup, q Queue, id int, c *Counter, workNS int) {
	defer wg.Done()
	spin := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if _, ok := q.Dequeue(); ok {
				atomic.AddUint64(&c.DeqOK, 1)
				busyWork(workNS)
				spin = 0
			} else {
				atomic.AddUint64(&c.DeqEmpty, 1)
				// light backoff to avoid burning CPU when empty
				spin++
				if spin < 50 {
					runtime.Gosched()
				} else {
					time.Sleep(time.Microsecond)
					if spin > 1000 {
						spin = 0
					}
				}
			}
		}
	}
}

func human(n uint64, dur time.Duration) string {
	opsPerSec := float64(n) / dur.Seconds()
	switch {
	case opsPerSec > 1e9:
		return fmt.Sprintf("%.2f Gops/s", opsPerSec/1e9)
	case opsPerSec > 1e6:
		return fmt.Sprintf("%.2f Mops/s", opsPerSec/1e6)
	case opsPerSec > 1e3:
		return fmt.Sprintf("%.2f Kops/s", opsPerSec/1e3)
	default:
		return fmt.Sprintf("%.2f ops/s", opsPerSec)
	}
}

func main() {
	var (
		queueType  = flag.String("q", "lock", "queue type: lock | ms")
		producers  = flag.Int("producers", 4, "number of producer goroutines")
		consumers  = flag.Int("consumers", 4, "number of consumer goroutines")
		duration   = flag.Duration("dur", 5*time.Second, "benchmark duration")
		workNS     = flag.Int("work", 0, "synthetic CPU nanos per successful op (simulate app work)")
		gomaxprocs = flag.Int("gomaxprocs", 0, "if >0, sets GOMAXPROCS")
		warmup     = flag.Duration("warmup", 500*time.Millisecond, "warmup time")
	)
	flag.Parse()

	if *gomaxprocs > 0 {
		runtime.GOMAXPROCS(*gomaxprocs)
	}

	// Reduce GC interference variance a bit
	debug.SetGCPercent(100)

	var q Queue
	switch *queueType {
	case "lock":
		q = NewTwoLockQueue()
	case "ms":
		q = NewMSQueue()
	default:
		panic("unknown -q type (use lock or ms)")
	}

	// Seed with some items so consumers don’t start on empty queue
	for i := 0; i < *consumers; i++ {
		q.Enqueue(i)
	}

	var total Counter
	var wg sync.WaitGroup

	// Warmup
	ctxW, cancelW := context.WithTimeout(context.Background(), *warmup)
	for i := 0; i < *producers; i++ {
		wg.Add(1)
		go runProducers(ctxW, &wg, q, i, &total, 0)
	}
	for i := 0; i < *consumers; i++ {
		wg.Add(1)
		go runConsumers(ctxW, &wg, q, i, &total, 0)
	}
	wg.Wait()
	cancelW()

	// Main run
	var counters []Counter
	ctx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()

	wg = sync.WaitGroup{}
	counters = make([]Counter, *producers+*consumers)

	for i := 0; i < *producers; i++ {
		wg.Add(1)
		go runProducers(ctx, &wg, q, i, &counters[i], *workNS)
	}
	for i := 0; i < *consumers; i++ {
		wg.Add(1)
		go runConsumers(ctx, &wg, q, i, &counters[*producers+i], *workNS)
	}
	wg.Wait()

	// Aggregate
	var agg Counter
	for i := range counters {
		agg.add(counters[i])
	}
	fmt.Printf("Queue: %s | P=%d C=%d | dur=%s | work/op=%dns\n", *queueType, *producers, *consumers, *duration, *workNS)
	fmt.Printf("Enqueue: %d  (%s)\n", agg.EnqOK, human(agg.EnqOK, *duration))
	fmt.Printf("Dequeue: %d  (%s)\n", agg.DeqOK, human(agg.DeqOK, *duration))
	fmt.Printf("Empty  : %d  (dequeue attempts when empty)\n", agg.DeqEmpty)
}
