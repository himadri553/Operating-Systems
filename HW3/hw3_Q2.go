package main

import (
	"flag"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

/***************
 * Common types
 ***************/

type List interface {
	Insert(key int) bool   // insert at head (returns true if success)
	Contains(key int) bool // lookup
	// (Delete omitted for simplicity—bench focuses on Insert vs Contains)
}

/**********************************************
 * 1) Coarse-grained (single-lock) linked list
 **********************************************/

type coarseNode struct {
	key  int
	next *coarseNode
}

type CoarseList struct {
	head *coarseNode
	mu   sync.Mutex
}

func NewCoarseList() *CoarseList {
	return &CoarseList{}
}

func (l *CoarseList) Insert(key int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	n := &coarseNode{key: key, next: l.head}
	l.head = n
	return true
}

func (l *CoarseList) Contains(key int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	for cur := l.head; cur != nil; cur = cur.next {
		if cur.key == key {
			return true
		}
	}
	return false
}

/*****************************************************
 * 2) Hand-over-hand (lock-coupling) linked list
 *    - Uses a sentinel head node so head pointer
 *      does not change (helps lock coupling).
 *****************************************************/

type hohNode struct {
	key  int
	next *hohNode
	mu   sync.Mutex
}

type HoHList struct {
	head *hohNode // sentinel: head.key is unused; data starts at head.next
}

func NewHoHList() *HoHList {
	// sentinel head (no data)
	return &HoHList{head: &hohNode{}}
}

// Insert at head: lock only the sentinel, splice new node
func (l *HoHList) Insert(key int) bool {
	l.head.mu.Lock()
	defer l.head.mu.Unlock()

	n := &hohNode{key: key, next: l.head.next}
	l.head.next = n
	return true
}

// Contains with lock coupling (no defers; explicit unlocks to avoid double-unlock)
func (l *HoHList) Contains(key int) bool {
	prev := l.head
	prev.mu.Lock()

	cur := prev.next
	for cur != nil {
		cur.mu.Lock()
		if cur.key == key {
			// Unlock both before returning
			cur.mu.Unlock()
			prev.mu.Unlock()
			return true
		}
		// Slide window: unlock prev, move forward
		prev.mu.Unlock()
		prev = cur
		cur = cur.next
	}

	// Unlock the last held lock (the sentinel if list was empty, or the last node visited)
	prev.mu.Unlock()
	return false
}

/**********************************
 * Benchmark / workload harness
 **********************************/

type config struct {
	impl         string        // "coarse", "hoh", or "both"
	workers      int           // goroutines
	writePercent int           // 0..100 (rest are reads)
	duration     time.Duration // per trial
	preload      int           // initial size
	keyspace     int           // random key range
	seed         int64
}

func parseFlags() config {
	var c config
	flag.StringVar(&c.impl, "impl", "both", "which impl to run: coarse | hoh | both")
	flag.IntVar(&c.workers, "workers", 8, "number of goroutines")
	flag.IntVar(&c.writePercent, "writePercent", 10, "percent of insert operations (0..100)")
	flag.DurationVar(&c.duration, "duration", 3*time.Second, "how long to run each trial")
	flag.IntVar(&c.preload, "preload", 20000, "how many keys to insert before running")
	flag.IntVar(&c.keyspace, "keyspace", 100000, "range of random keys used by workers")
	flag.Int64Var(&c.seed, "seed", time.Now().UnixNano(), "random seed")
	flag.Parse()
	return c
}

func preloadList(L List, n, keyspace int, seed int64) {
	r := rand.New(rand.NewSource(seed))
	for i := 0; i < n; i++ {
		L.Insert(r.Intn(keyspace))
	}
}

type result struct {
	ops uint64
}

func runTrial(name string, L List, c config) result {
	var ops uint64
	stop := time.Now().Add(c.duration)

	var wg sync.WaitGroup
	wg.Add(c.workers)

	// Give each worker its own RNG to avoid contention
	for w := 0; w < c.workers; w++ {
		wseed := c.seed + int64(w)*101
		r := rand.New(rand.NewSource(wseed))
		go func() {
			defer wg.Done()
			for time.Now().Before(stop) {
				k := r.Intn(c.keyspace)
				// choose op
				if r.Intn(100) < c.writePercent {
					L.Insert(k)
				} else {
					L.Contains(k)
				}
				atomic.AddUint64(&ops, 1)
			}
		}()
	}

	wg.Wait()
	return result{ops: ops}
}

func main() {
	c := parseFlags()
	fmt.Printf("Concurrent Linked List Benchmark\n")
	fmt.Printf("impl=%s workers=%d write%%=%d duration=%s preload=%d keyspace=%d\n\n",
		c.impl, c.workers, c.writePercent, c.duration, c.preload, c.keyspace)

	run := func(name string, newList func() List) {
		L := newList()
		preloadList(L, c.preload, c.keyspace, c.seed)
		res := runTrial(name, L, c)
		opsPerSec := float64(res.ops) / c.duration.Seconds()
		fmt.Printf("%-12s  total_ops=%d  ops/sec=%.0f\n", name, res.ops, opsPerSec)
	}

	switch c.impl {
	case "coarse":
		run("coarse-lock", func() List { return NewCoarseList() })
	case "hoh":
		run("hand-over", func() List { return NewHoHList() })
	case "both":
		run("coarse-lock", func() List { return NewCoarseList() })
		run("hand-over", func() List { return NewHoHList() })
	default:
		fmt.Println("unknown -impl; use coarse | hoh | both")
	}
}
