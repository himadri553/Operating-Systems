# EECE 4811 - Operating Systems

#Group Members
Himadri Saha, Ashwin Srinivasan, Yaritza Sanchez

# Language
Go programming language

# How to Run the Programs

We used Visual Studio IDE with the Go installer.

    1. Open the project folder in VS Code, then run the .go files in the terminal
        
# HW0
        Question 1 - run in terminal: go run producer-consumer.go or press the run and debugg button

        Output:

            DAP server listening at: 127.0.0.1:55160
            Type 'dlv help' for list of commands.
            Our output in VS Code:
            Producer: 1
            Consumer: 1
            Producer: 2
            Consumer: 2
            Producer: 3
            Consumer: 3
            Producer: 4
            Consumer: 4
            Producer: 5
            Consumer: 5
            Process 16444 has exited with status 0
            Detaching

        Question 2 - run in terminal: go run stack.go or press the run and debugg button

        Output:
                
            DAP server listening at: 127.0.0.1:55009
            Type 'dlv help' for list of commands.
            Popped: 30
            Popped: 20
            Popped: 10
            Error: Stack underflow
            Popped: -1
            Process 19700 has exited with status 0
            Detaching

        Design of the Program
    
            Q1: Producer and Consumer
                The parent process is the producer. 
                It spawns a child process with the --role=consumer flag.

            Pipes:

            Producer → Consumer: sends numbers (stdin).
            Consumer → Producer: sends ACK messages (stderr).
            Consumer → Terminal: prints outputs (stdout).
            The producer waits for an ACK before sending the next number → ensures correct order.
            Uses processes + pipes (threads).

            Q2: Stack
                Implemented with a fixed array of size 100.
                Push: adds a value to the top of the stack, checks for overflow.
                Pop: removes and returns the top value, checks for underflow.
                Demo shows pushing 10, 20, 30 then popping four times → last one triggers underflow.

        Dependencies

            Only Go standard library: fmt, os, os/exec, bufio, strconv.
            No extra libraries needed.

# HW1
        Question 1 - attached in Word doc.
        
        
        Question 2 - run in terminal: run hw1_q2.go or press the run and debug button

         - Process-based (parent/child with pipes)
         - Goroutine-based (single process, channels)
         - Includes a simple benchmark harness.
         
        Dependencies:

            Only Go standard library: fmt, os, os/exec, bufio, strconv.
            No extra libraries needed

        Design of the Program:
        - Demonstrates and benchmarks two ways of implementing a producer–consumer system: using goroutines with channels or using separate            OS processes with pipes. In goroutine mode, the main function (producer) sends integers to a consumer goroutine through a channel
        - In process mode, the parent process spawns a child copy of itself with a special flag (--role=consumer), then sends numbers                  through the child’s stdin and waits for "ACK\n" responses on the child’s stderr. 
        - The '--quiet' flag suppresses prints to avoid I/O overhead, and the '--bench' flag runs trials in both modes to collect                      average, best, and standard deviation of runtimes.
        
# HW4
        Question 1 - attached in github.
        
        
        Question 2 - run in terminal

Two-lock queue (Figure 29.9):

Dummy node so head always points to a node whose next is the first real element. Tail lock protects enqueues; head lock protects dequeues.
Enqueues don’t block dequeues (and vice versa), but enqueues contend with each other on the tail lock, and dequeues contend on head lock.

Michael & Scott lock-free queue:

Head and tail are "atomic.Pointer" to nodes; each node’s next is also atomic.Pointer.
Enqueue: link new node with CAS(tail.next, nil, n) then try to swing tail to n. If tail.next != nil, help advance the tail.
Dequeue: read head, tail, head.next; if empty, return false; if head == tail, help advance tail; else CAS(head, head, head.next).

Benchmarks:

-  SPSC (Single-Producer / Single-Consumer):
Very low contention; minimal pointer sharing.
Expectation: the two-lock version can be as fast or faster (fewer atomics, zero CAS retries). Lock-free’s CAS loop overhead provides little benefit here.

- MPSC (Many Producers / Single Consumer)
Tail is the hotspot.
Two-lock: all producers serialize on the tail mutex → bottleneck.
Lock-free: producers still contend on tail.next CAS, but there’s no blocking; retriers can make progress once the cache line updates, often improving throughput under high producer counts.

- SPMC (Single Producer / Many Consumers)
Head is the hotspot.
Two-lock: consumers serialize on the head mutex.
Lock-free: consumers contend via CAS on head but avoid blocking; expected to do better as consumer count grows.

- MPMC (Many / Many)
Both ends hot; maximum contention.
Two-lock has two independent bottlenecks; lock-free still contends but can reduce convoying and avoid priority inversion, often leading to better scaling with CPU count.
