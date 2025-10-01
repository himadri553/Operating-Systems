package main

import (
	"flag"
	"fmt"
	"math"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

/*
We compare two locks:

1) TicketLock  — fair (first-come, first-served). Uses atomic.AddUint64
                 as a "fetch-and-add" to hand out ticket numbers.

2) CASLock     — unfair spin lock. Uses Compare-And-Swap on a 0/1 flag.

We measure how long each goroutine waits to acquire the lock
("wait time") under different amounts of contention.
*/

type Lock interface {
	Lock()
	Unlock()
}

/* ---------------- CAS spin lock (unfair) ---------------- */

type CASLock struct {
	state int32 // 0 = unlocked, 1 = locked
}

func (l *CASLock) Lock() {
	// Try to change state from 0 -> 1.
	// If it fails, give the scheduler a chance and try again.
	for !atomic.CompareAndSwapInt32(&l.state, 0, 1) {
		runtime.Gosched()
	}
}

func (l *CASLock) Unlock() {
	// Set back to unlocked.
	atomic.StoreInt32(&l.state, 0)
}

/* ---------------- Ticket lock (FIFO / fair) ---------------- */

type TicketLock struct {
	next       uint64 // next ticket number to give out
	nowServing uint64 // ticket number currently allowed to enter
}

func (l *TicketLock) Lock() {
	// atomic.AddUint64 returns the NEW value, but fetch-and-add returns the OLD one.
	// So we subtract 1 to get "my" ticket number.
	my := atomic.AddUint64(&l.next, 1) - 1

	// Wait until it's my turn.
	for atomic.LoadUint64(&l.nowServing) != my {
		runtime.Gosched()
	}
}

func (l *TicketLock) Unlock() {
	// Let the next ticket in.
	atomic.AddUint64(&l.nowServing, 1)
}

/* ---------------- Critical-section "work" ---------------- */

// busyUS burns ~us microseconds doing nothing.
// We use this to control how long the lock is held.
func busyUS(us int) {
	if us <= 0 {
		return
	}
	end := time.Now().Add(time.Duration(us) * time.Microsecond)
	for time.Now().Before(end) {
		// spin
	}
}

/* ---------------- Simple stats helpers ---------------- */

type Summary struct {
	N      int
	MeanNS float64
	P50NS  float64
	P95NS  float64
	MaxNS  float64
}

func summarize(ds []time.Duration) Summary {
	n := len(ds)
	if n == 0 {
		return Summary{}
	}

	vals := make([]float64, n)
	var sum, max float64
	for i, d := range ds {
		v := float64(d.Nanoseconds())
		vals[i] = v
		sum += v
		if v > max {
			max = v
		}
	}
	sort.Float64s(vals)

	// Return the q-th percentile with simple linear interpolation.
	percentile := func(q float64) float64 {
		if n == 1 {
			return vals[0]
		}
		pos := q * float64(n-1)
		lo := int(math.Floor(pos))
		hi := int(math.Ceil(pos))
		if lo == hi {
			return vals[lo]
		}
		f := pos - float64(lo)
		return vals[lo]*(1-f) + vals[hi]*f
	}

	return Summary{
		N:      n,
		MeanNS: sum / float64(n),
		P50NS:  percentile(0.50),
		P95NS:  percentile(0.95),
		MaxNS:  max,
	}
}

/* ---------------- Benchmark runner ---------------- */

// run starts G goroutines. Each goroutine:
//   - tries to lock,
//   - records how long it waited,
//   - does a tiny bit of work inside the lock,
//   - unlocks.
// We collect all wait times and summarize them.
func run(lock Lock, goroutines, iters, csUS int) Summary {
	var wg sync.WaitGroup
	wg.Add(goroutines)

	startGate := make(chan struct{})           // used to start everyone at once
	results := make(chan []time.Duration, goroutines)

	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			local := make([]time.Duration, 0, iters)

			<-startGate // wait until we open the gate
			for i := 0; i < iters; i++ {
				t0 := time.Now()
				lock.Lock()
				wait := time.Since(t0)

				busyUS(csUS) // hold the lock for ~csUS microseconds

				lock.Unlock()
				local = append(local, wait)
			}
			results <- local
		}()
	}

	close(startGate) // start all goroutines together
	wg.Wait()
	close(results)

	// merge all wait times
	all := make([]time.Duration, 0, goroutines*iters)
	for r := range results {
		all = append(all, r...)
	}
	return summarize(all)
}

/* ---------------- main + flags ---------------- */

func main() {
	var (
		lockType   = flag.String("type", "ticket", "lock type: ticket | cas")
		goroutines = flag.Int("goroutines", 8, "number of goroutines contending")
		iters      = flag.Int("iters", 100000, "lock acquisitions per goroutine")
		csUS       = flag.Int("csus", 2, "critical-section time (microseconds)")
		gmp        = flag.Int("gomaxprocs", runtime.NumCPU(), "number of CPUs to use")
	)
	flag.Parse()

	// Limit how many CPUs the Go scheduler uses.
	runtime.GOMAXPROCS(*gmp)

	// Pick the lock type.
	var l Lock
	switch *lockType {
	case "ticket":
		l = &TicketLock{}
	case "cas":
		l = &CASLock{}
	default:
		panic("unknown -type (use 'ticket' or 'cas')")
	}

	// Short warmup so the scheduler settles a bit.
	_ = run(l, 2, 2000, 1)

	// Real run.
	s := run(l, *goroutines, *iters, *csUS)

	fmt.Printf("Lock=%s  G=%d  iters=%d  cs=%dus  GOMAXPROCS=%d\n",
		*lockType, *goroutines, *iters, *csUS, *gmp)
	fmt.Printf("Wait (ns): mean=%.0f  p50=%.0f  p95=%.0f  max=%.0f  (N=%d)\n",
		s.MeanNS, s.P50NS, s.P95NS, s.MaxNS, s.N)
}
