package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p golParams, d distributorChans, alive chan []cell) {

	// Create the 2D slice to store the world.
	world := make([][]byte, p.imageHeight)
	for i := range world {
		world[i] = make([]byte, p.imageWidth)
	}

	// Request the io goroutine to read in the image with the given filename.
	d.io.command <- ioInput
	d.io.filename <- strings.Join([]string{strconv.Itoa(p.imageWidth), strconv.Itoa(p.imageHeight)}, "x")

	// The io goroutine sends the requested image byte by byte, in rows.
	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			val := <-d.io.inputVal
			if val != 0 {
				// fmt.Println("Alive cell at", x, y)
				world[y][x] = val
			}
		}
	}

	workerHeight := p.imageHeight / p.threads
	bigWorkers := p.imageHeight % p.threads

	// Send the section of the image to workers byte by byte, in rows.
	startY := 0
	workerHeight++
	for i := 0; i < p.threads; i++ {
		if i == bigWorkers {
			workerHeight--
		}
		for yd := -1; yd <= workerHeight; yd++ {
			y := (startY + yd + p.imageHeight) % p.imageHeight
			for x := 0; x < p.imageWidth; x++ {
				d.workerVals[i] <- world[y][x]
			}
		}
		startY += workerHeight
	}

	pause := false
	terminate := false

	// 2b - Print alive cells every 2 seconds
	ticker := time.NewTicker(2 * time.Second)
	go func() {
		for {
			<-ticker.C
			if pause == false {
				for i := 0; i < p.threads; i++ {
					d.workerCommands[i] <- workerSendAlive
				}
				totalAlive := 0
				for i := 0; i < p.threads; i++ {
					totalAlive += <-d.aliveWorkers
				}
				fmt.Println("alive:", totalAlive)
			}
		}

	}()

	// Calculate the new state of Game of Life after the given number of turns.
Turns:
	for turns := 0; turns < p.turns && terminate == false; {
		// fmt.Println(turns)
		//Key press handler
		select {
		case key := <-d.key:
			switch key {
			case 's':
				fmt.Println("Make current PGM")
				for i := 0; i < p.threads; i++ {
					d.workerCommands[i] <- workerCurentPGM
				}
				//Receive world from workers
				startY = 0
				workerHeight++
				for i := 0; i < p.threads; i++ {
					if i == bigWorkers {
						workerHeight--
					}
					for yd := 0; yd < workerHeight; yd++ {
						y := startY + yd
						for x := 0; x < p.imageWidth; x++ {
							world[y][x] = <-d.workerVals[i]
						}
					}
					startY += workerHeight
				}
				generatePGM(p, d, world)

			case 'p':
				fmt.Println("Paused")
				pause = true
				var resume rune
				for resume != 'p' {
					resume = <-d.key
				}
				pause = false
				fmt.Println("Continuing")

			case 'q':
				fmt.Println("Terminate and generate PGM")
				break Turns
			}
		default:
			turns++
			for i := 0; i < p.threads; i++ {
				d.workerNextTurns[i] <- 0
			}
		}
	}

	for i := 0; i < p.threads; i++ {
		d.workerCommands[i] <- workerQuit
	}

	//Receive world from workers
	startY = 0
	workerHeight++
	for i := 0; i < p.threads; i++ {
		if i == bigWorkers {
			workerHeight--
		}
		for yd := 0; yd < workerHeight; yd++ {
			y := startY + yd
			for x := 0; x < p.imageWidth; x++ {
				world[y][x] = <-d.workerVals[i]
			}
		}
		startY += workerHeight
	}

	//Send world to pgm one byte at a time
	generatePGM(p, d, world)

	// Create an empty slice to store coordinates of cells that are still alive after p.turns are done.
	var finalAlive []cell

	// Go through the world and append the cells that are still alive.
	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			if world[y][x] != 0 {
				finalAlive = append(finalAlive, cell{x: x, y: y})
			}
		}
	}

	// Make sure that the Io has finished any output before exiting.
	d.io.command <- ioCheckIdle
	<-d.io.idle

	// Return the coordinates of cells that are still alive.
	alive <- finalAlive
}

func generatePGM(p golParams, d distributorChans, world [][]uint8) {
	d.io.command <- ioOutput
	d.io.filename <- strings.Join([]string{strconv.Itoa(p.imageWidth), strconv.Itoa(p.imageHeight)}, "x") + "_" + strconv.Itoa(p.turns)

	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			d.io.outputVal <- world[y][x]
		}
	}
}

func worker(p golParams, val, topHalo, bottomHalo, nextTurn chan uint8, alive chan int, commandChan chan workerCommand, height int, num int) {

	// Create the 2D slice to store the section of the world.
	world := make([][]byte, height+2)
	for i := range world {
		world[i] = make([]byte, p.imageWidth)
	}

	tempWorld := make([][]byte, height+2)
	for i := range tempWorld {
		tempWorld[i] = make([]byte, p.imageWidth)
	}

	// Receive the section of the image byte by byte, in rows.
	for y := 0; y < height+2; y++ {
		for x := 0; x < p.imageWidth; x++ {
			world[y][x] = <-val
		}
	}

	numAlive := 0

Turns:
	for {
		select {
		case command := <-commandChan:
			switch command {
			case workerCurentPGM:
				for y := 1; y <= height; y++ {
					for x := 0; x < p.imageWidth; x++ {
						val <- world[y][x]
					}
				}
			case workerQuit:
				break Turns
			case workerSendAlive:
				alive <- numAlive
			}
		case <-nextTurn:
			numAlive = 0

			for y := 1; y <= height; y++ {
				for x := 0; x < p.imageWidth; x++ {

					//Count number of alive neighbours
					alive := 0
					for y1 := y - 1; y1 <= y+1; y1++ {
						for x1 := x - 1; x1 <= x+1; x1++ {
							if x != x1 || y != y1 {
								if world[y1][(x1+p.imageWidth)%p.imageWidth] == 0xFF {
									alive++
								}
							}
						}
					}

					//Decide whether cell lives or dies
					if world[y][x] == 0xFF {
						if alive < 2 || alive > 3 {
							tempWorld[y][x] = 0
						} else {
							tempWorld[y][x] = 0xFF
							numAlive++
						}
					} else if world[y][x] == 0 && alive == 3 {
						tempWorld[y][x] = 0xFF
						numAlive++
					}

				}
			}

			for y := 1; y <= height; y++ {
				for x := 0; x < p.imageWidth; x++ {
					world[y][x] = tempWorld[y][x]
				}
			}

			//Send/receive halos between neighbouring workers
			for x := 0; x < p.imageWidth; x++ {
				if num%2 == 0 {
					topHalo <- world[1][x]
					bottomHalo <- world[height][x]
				}
				world[height+1][x] = <-bottomHalo
				world[0][x] = <-topHalo
				if num%2 != 0 {
					topHalo <- world[1][x]
					bottomHalo <- world[height][x]
				}
			}
		}
	}

	for y := 1; y <= height; y++ {
		for x := 0; x < p.imageWidth; x++ {
			val <- world[y][x]
		}
	}
}

func printGrid(world [][]byte) {
	width := len(world[0])
	height := len(world)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			fmt.Print(world[y][x])
			if x >= width-1 {
				fmt.Println()
			}
		}
	}
	fmt.Println()
}
