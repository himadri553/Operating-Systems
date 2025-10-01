/*
	EECE 4811 - Operating Systems
	HW0 - Question 1
	Himadri Saha, Ashwin Srinivasan, Yaritza Sanchez

	List of processes
	- producer (parent)
	- consumer (child)

	List of pipes
	- consumerStdin: Produer to Consumer, sends numbers (1-5)
	- consumerAck: Consumer to Producer, sends ack message after each number (Producer to wait)
	- cmd.Stdout: Consumer to Terminal, just to redirict Consumer print statements
*/

/*Imports, packages and constants*/
package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

const roleFlag = "--role=consumer"

func main() {
	/*
		Run as a prodcuer first
		Then, when producer function runs code with consumer flag, run consumer
	*/
	if len(os.Args) > 1 && os.Args[1] == roleFlag {
		consumer()
		return
	}
	producer()
}

func producer() {
	// Run program with consumer flag
	cmd := exec.Command(os.Args[0], roleFlag)

	// Set up pipes
	consumerStdin, _ := cmd.StdinPipe() // Producer writes here
	consumerAck, _ := cmd.StderrPipe()  // Producer reads ACK messages from stderr
	cmd.Stdout = os.Stdout              // Connects consumers stdout to terminal
	cmd.Start()                         // Start child process

	ackReader := bufio.NewScanner(consumerAck)

	// Print "Producer: n" to consumerStdin and wait from ACK from Stderr
	for n := 1; n <= 5; n++ {
		fmt.Printf("Producer: %d\n", n)
		fmt.Fprintln(consumerStdin, n)
		ackReader.Scan()
	}

	// Close pipes and wait for child process to finish
	consumerStdin.Close()
	cmd.Wait()
}

func consumer() {
	// Read numbers fed by producer
	in := bufio.NewScanner(os.Stdin)

	// Loop until stdin closes
	for in.Scan() {
		n, err := strconv.Atoi(in.Text())
		if err != nil {
			continue
		}
		fmt.Printf("Consumer: %d\n", n) // Print straight to terminal
		fmt.Fprintln(os.Stderr, "ACK")  // Send ACK message
	}
}