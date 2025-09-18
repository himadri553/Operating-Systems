package main

import (
	"fmt" //library
)

// Stack definition
type Stack struct {
	arr [100]int
	top int
}

// Initialize a new stack
func NewStack() *Stack {
	return &Stack{top: -1}
}

// Push function
func (s *Stack) Push(value int) {
	if s.top >= len(s.arr)-1 {
		fmt.Println("Error: Stack overflow") //run out of values to push  out
		return
	}
	s.top++
	s.arr[s.top] = value //end push function //end push function
}

// Pop function
func (s *Stack) Pop() int {
	if s.top < 0 {
		fmt.Println("Error: Stack underflow") //run out of values to push  out //run out of values to push  out
		return -1 // sentinel value
	}
	val := s.arr[s.top]
	s.top--
	return val //end pop function
}

// Demo
func main() {
	stack := NewStack()

	stack.Push(10)
	stack.Push(20)
	stack.Push(30)

	fmt.Println("Popped:", stack.Pop()) // 30
	fmt.Println("Popped:", stack.Pop()) // 20
	fmt.Println("Popped:", stack.Pop()) // 10
	fmt.Println("Popped:", stack.Pop()) // Underflow
}