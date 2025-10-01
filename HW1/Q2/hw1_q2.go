// EECE 4811 - Operating Systems
// HW0/HW1: One Producer / One Consumer
// Himadri Saha, Ashwin Srinivasan, Yaritza Sanchez
// - Process-based (parent/child with pipes)
// - Goroutine-based (single process, channels)
// Includes a simple benchmark harness.
//
// Notes:
// - For fair timing, use --quiet and large --n.
// ---buf only affects goroutine mode (channel capacity).

package main

import (
	"bufio"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"time"
)

const roleFlag = "--role=consumer"

var (
	mode   = flag.String("mode", "goroutine", "process | goroutine")
	n      = flag.Int("n", 5, "count of numbers to exchange")
	trials = flag.Int("trials", 3, "benchmark trials (when --bench)")
	bufSz  = flag.Int("buf", 0, "channel buffer size (goroutine mode only)")
	quiet  = flag.Bool("quiet", false, "suppress per-item prints for timing")
	bench  = flag.Bool("bench", false, "run benchmark comparing modes")
)

func main() {
	flag.Parse()

	// Child process path
	if len(os.Args) > 1 && os.Args[1] == roleFlag {
		// parse optional quiet flag passed to child
		childQuiet := false
		for _, a := range os.Args[2:] {
			if a == "--quiet" || a == "--quiet=true" {
				childQuiet = true
			}
		}
		if err := consumerProcess(childQuiet); err != nil {
			fmt.Fprintln(os.Stderr, "consumer error:", err)
			os.Exit(1)
		}
		return
	}

	// Top-level runner / benchmarker
	if *bench {
		runBenchmarks(*n, *trials, *bufSz)
		return
	}

	switch *mode {
	case "process":
		dur, err := runProcess(*n, *quiet)
		if err != nil {
			fmt.Fprintln(os.Stderr, "process mode error:", err)
			os.Exit(1)
		}
		fmt.Printf("process mode: n=%d elapsed=%v\n", *n, dur)
	case "goroutine":
		dur := runGoroutine(*n, *bufSz, *quiet)
		fmt.Printf("goroutine mode: n=%d buf=%d elapsed=%v\n", *n, *bufSz, dur)
	default:
		fmt.Fprintln(os.Stderr, "unknown --mode (use process|goroutine)")
		os.Exit(2)
	}
}

// Goroutine mode (HW1)

func runGoroutine(N, chanBuf int, quiet bool) time.Duration {
	runtime.GOMAXPROCS(runtime.NumCPU())

	data := make(chan int, chanBuf)
	ack := make(chan struct{})

	start := time.Now()

	// Consumer goroutine
	go func() {
		for x := range data {
			if !quiet && x <= 5 {
				fmt.Printf("Consumer: %d\n", x)
			}
			ack <- struct{}{} // simple sync (like your ACK line)
		}
	}()

	// Producer (main goroutine)
	for i := 1; i <= N; i++ {
		if !quiet && i <= 5 {
			fmt.Printf("Producer: %d\n", i)
		}
		data <- i
		<-ack
	}
	close(data)

	return time.Since(start)
}


// Process mode (HW0, refined)
// Parent = producer, Child = consumer via exec + pipes

func runProcess(N int, quiet bool) (time.Duration, error) {
	cmd := exec.Command(os.Args[0], roleFlag)
	if quiet {
		cmd.Args = append(cmd.Args, "--quiet")
	}

	// Pipes: parent writes to child's stdin, reads ACKs from child's stderr
	consumerStdin, err := cmd.StdinPipe()
	if err != nil {
		return 0, err
	}
	consumerAck, err := cmd.StderrPipe()
	if err != nil {
		return 0, err
	}
	// Child stdout goes to our stdout (useful for demos; quiet suppresses prints anyway)
	cmd.Stdout = os.Stdout

	if err := cmd.Start(); err != nil {
		return 0, err
	}

	ackReader := bufio.NewReader(consumerAck)
	writer := bufio.NewWriterSize(consumerStdin, 64*1024)

	start := time.Now()
	for i := 1; i <= N; i++ {
		if !quiet && i <= 5 {
			fmt.Printf("Producer: %d\n", i)
		}
		// Write number + newline for child's scanner/reader
		_, _ = writer.WriteString(strconv.Itoa(i))
		_ = writer.WriteByte('\n')
		// Flush promptly so child sees it (line-buffered protocol)
		_ = writer.Flush()

		// Wait for "ACK\n"
		if _, err := ackReader.ReadString('\n'); err != nil {
			return 0, err
		}
	}
	_ = consumerStdin.Close()
	if err := cmd.Wait(); err != nil {
		return 0, err
	}
	elapsed := time.Since(start)
	return elapsed, nil
}

// Child process entry: reads numbers from stdin, emits "ACK\n" on stderr.
func consumerProcess(quiet bool) error {
	in := bufio.NewScanner(os.Stdin)
	outAck := bufio.NewWriterSize(os.Stderr, 64*1024)

	for in.Scan() {
		txt := in.Text()
		n, err := strconv.Atoi(txt)
		if err != nil {
			continue
		}
		if !quiet && n <= 5 {
			fmt.Printf("Consumer: %d\n", n)
		}
		if _, err := outAck.WriteString("ACK\n"); err != nil {
			return err
		}
	}
	if err := in.Err(); err != nil {
		return err
	}
	return outAck.Flush()
}


// Benchmark harness

type stat struct {
	avg, best, std time.Duration
	all            []time.Duration
}

func runBenchmarks(N, Trials, chanBuf int) {
	fmt.Printf("Benchmarking with n=%d, trials=%d, quiet=%v\n", N, Trials, *quiet)
	fmt.Println("Tip: run with --quiet for fair timing (I/O is expensive).")

	pStat := doTrials("process", Trials, func() (time.Duration, error) { return runProcess(N, *quiet) })
	gStat := doTrials("goroutine", Trials, func() (time.Duration, error) { return runGTrial(N, chanBuf, *quiet) })

	fmt.Printf("\nResults (lower is better):\n")
	printStat("process   ", pStat)
	printStat("goroutine ", gStat)
}

func gTrialOnce(N, buf int, quiet bool) time.Duration { return runGoroutine(N, buf, quiet) }
func runGTrial(N, buf int, quiet bool) (time.Duration, error) {
	return gTrialOnce(N, buf, quiet), nil
}

func doTrials(label string, Trials int, fn func() (time.Duration, error)) stat {
	durs := make([]time.Duration, 0, Trials)
	var best time.Duration
	best = time.Duration(math.MaxInt64)

	for t := 0; t < Trials; t++ {
		// light GC to reduce noise between trials
		runtime.GC()
		time.Sleep(20 * time.Millisecond)

		d, err := fn()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s trial %d error: %v\n", label, t+1, err)
			continue
		}
		durs = append(durs, d)
		if d < best {
			best = d
		}
	}

	return stat{
		avg:  average(durs),
		best: best,
		std:  stddev(durs),
		all:  durs,
	}
}

func printStat(name string, s stat) {
	if len(s.all) == 0 {
		fmt.Printf("%s: no successful trials\n", name)
		return
	}
	fmt.Printf("%s  avg=%v  best=%v  std=%v  samples=%v\n", name, s.avg, s.best, s.std, s.all)
}

func average(d []time.Duration) time.Duration {
	if len(d) == 0 {
		return 0
	}
	var sum time.Duration
	for _, v := range d {
		sum += v
	}
	return sum / time.Duration(len(d))
}

func stddev(d []time.Duration) time.Duration {
	if len(d) <= 1 {
		return 0
	}
	avg := float64(average(d))
	var ss float64
	for _, v := range d {
		dx := float64(v) - avg
		ss += dx * dx
	}
	return time.Duration(math.Sqrt(ss / float64(len(d)-1)))
}

