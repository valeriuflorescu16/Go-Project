package main

import (
	"flag"
	"fmt"
)

// golParams provides the details of how to run the Game of Life and which image to load.
type golParams struct {
	turns       int
	threads     int
	imageWidth  int
	imageHeight int
}

// ioCommand allows requesting behaviour from the io (pgm) goroutine.
type ioCommand uint8

// This is a way of creating enums in Go.
// It will evaluate to:
//		ioOutput 	= 0
//		ioInput 	= 1
//		ioCheckIdle = 2
const (
	ioOutput ioCommand = iota
	ioInput
	ioCheckIdle
)

type workerCommand uint8

const (
	workerCurentPGM workerCommand = iota
	workerQuit
	workerSendAlive
)

// cell is used as the return type for the testing framework.
type cell struct {
	x, y int
}

// distributorToIo defines all chans that the distributor goroutine will have to communicate with the io goroutine.
// Note the restrictions on chans being send-only or receive-only to prevent bugs.
type distributorToIo struct {
	command chan<- ioCommand
	idle    <-chan bool

	filename  chan<- string
	inputVal  <-chan uint8
	outputVal chan<- uint8
}

// ioToDistributor defines all chans that the io goroutine will have to communicate with the distributor goroutine.
// Note the restrictions on chans being send-only or receive-only to prevent bugs.
type ioToDistributor struct {
	command <-chan ioCommand
	idle    chan<- bool

	filename  <-chan string
	inputVal  chan<- uint8
	outputVal <-chan uint8
}

// distributorChans stores all the chans that the distributor goroutine will use.
type distributorChans struct {
	io              distributorToIo
	key             <-chan rune
	workerVals      []chan uint8
	aliveWorkers    chan int
	workerCommands  []chan workerCommand
	workerNextTurns []chan uint8
}

// ioChans stores all the chans that the io goroutine will use.
type ioChans struct {
	distributor ioToDistributor
}

// gameOfLife is the function called by the testing framework.
// It makes some channels and starts relevant goroutines.
// It places the created channels in the relevant structs.
// It returns an array of alive cells returned by the distributor.
func gameOfLife(p golParams, keyChan <-chan rune) []cell {
	fmt.Println("----START", p.imageHeight, p.threads)

	var dChans distributorChans
	var ioChans ioChans

	ioCommand := make(chan ioCommand)
	dChans.io.command = ioCommand
	ioChans.distributor.command = ioCommand

	ioIdle := make(chan bool)
	dChans.io.idle = ioIdle
	ioChans.distributor.idle = ioIdle

	ioFilename := make(chan string)
	dChans.io.filename = ioFilename
	ioChans.distributor.filename = ioFilename

	inputVal := make(chan uint8)
	dChans.io.inputVal = inputVal
	ioChans.distributor.inputVal = inputVal

	outputVal := make(chan uint8)
	dChans.io.outputVal = outputVal
	ioChans.distributor.outputVal = outputVal

	dChans.key = keyChan

	aliveWorkers := make(chan int)
	dChans.aliveWorkers = aliveWorkers

	var workerCommands []chan workerCommand
	var workerVals []chan uint8
	var haloChans []chan uint8
	var workerNextTurns []chan uint8
	for i := 0; i < p.threads; i++ {
		workerCommands = append(workerCommands, make(chan workerCommand))
		workerVals = append(workerVals, make(chan uint8))
		haloChans = append(haloChans, make(chan uint8))
		workerNextTurns = append(workerNextTurns, make(chan uint8))
	}
	dChans.workerCommands = workerCommands
	dChans.workerVals = workerVals
	dChans.workerNextTurns = workerNextTurns

	workerHeight := p.imageHeight/p.threads + 1
	numBigWorkers := p.imageHeight % p.threads
	for i := 0; i < p.threads; i++ {
		if i == numBigWorkers {
			workerHeight--
		}
		go worker(p, workerVals[i], haloChans[i], haloChans[(i+1)%p.threads], workerNextTurns[i], aliveWorkers, workerCommands[i], workerHeight, i)
	}

	aliveCells := make(chan []cell)
	go distributor(p, dChans, aliveCells)
	go pgmIo(p, ioChans)

	alive := <-aliveCells
	return alive
}

// main is the function called when starting Game of Life with 'make gol'
// Do not edit until Stage 2.
func main() {
	var params golParams
	keyChan := make(chan rune)

	flag.IntVar(
		&params.threads,
		"t",
		8,
		"Specify the number of worker threads to use. Defaults to 8.")

	flag.IntVar(
		&params.imageWidth,
		"w",
		512,
		"Specify the width of the image. Defaults to 512.")

	flag.IntVar(
		&params.imageHeight,
		"h",
		512,
		"Specify the height of the image. Defaults to 512.")

	flag.Parse()

	params.turns = 1000000000000

	startControlServer(params)
	go getKeyboardCommand(keyChan)
	gameOfLife(params, keyChan)
	StopControlServer()
}
