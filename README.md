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
        Question 1 - Answer   attached in Word doc.
        
        
        Question 2 - run in terminal: run hw1_q2.go or press the run and debug button

         - Process-based (parent/child with pipes)
         - Goroutine-based (single process, channels)
         - Includes a simple benchmark harness.
         
        Dependencies

            Only Go standard library: fmt, os, os/exec, bufio, strconv.
            No extra libraries needed


        
