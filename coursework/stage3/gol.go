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

	var alivecells int
	// The io goroutine sends the requested image byte by byte, in rows.
	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			val := <-d.io.inputVal
			if val != 0 {
				fmt.Println("Alive cell at", x, y)
				world[y][x] = val
			}
		}
	}

	//2b - Print alive cells every 2 seconds
	ticker := time.NewTicker(2 * time.Second)
	alivechannel := make(chan int, 1)
	var pause = false

	go func() {
		for {
			select {
			case <-ticker.C:
				alivechannel <- alivecells
				printalive := <-alivechannel
				if pause == false {
					fmt.Printf("%v\n", printalive)
				}
			}
		}
	}()

	// Calculate the new state of Game of Life after the given number of turns.

	workerHeight := p.imageHeight / p.threads
	bigWorkers := p.imageHeight % p.threads

	terminate := false

	for turns := 0; turns < p.turns && terminate == false; turns++ {

		alivecells = 0

		for y := 0; y < p.imageHeight; y++ {
			for x := 0; x < p.imageWidth; x++ {
				if world[y][x] != 0 {
					alivecells++
				}
			}
		}

		//fmt.Printf("%v\n", alivecells)
		//nocellsalive := make(chan int, 1)
		//nocellsalive <- cellsalive

		/*if terminate == false {
			time.Sleep(2 * time.Second)
			nocellsalive <- cellsalive
			printalive := <-nocellsalive
			fmt.Printf("%v\n", printalive)

		} */

		//go aliveTimer(nocellsalive, cellsalive)

		//Key press handler
		select {
		case key := <-d.key:
			switch key {
			case 's':
				fmt.Println("Make current PGM")
				generatePGM(p, d, world)
			case 'p':
				fmt.Println("Paused")
				var resume rune
				pause = true
				for resume != 'p' {
					resume = <-d.key
					pause = false
				}
				fmt.Println("Continuing")
			case 'q':
				fmt.Println("Terminate and generate PGM")
				terminate = true
			}
		//case s := <-nocellsalive:
		//	fmt.Printf("%v\n", s)
		default:
			tempWorld := make([][]byte, p.imageHeight)
			for i := range tempWorld {
				tempWorld[i] = make([]byte, p.imageWidth)
			}

			//WORKER CODE

			// Send the section of the image to workers byte by byte, in rows.
			y := 0

			for i := 0; i < bigWorkers; i++ {
				// if turns == 0 {
				// 	fmt.Println("Dist sending big to worker", i)
				// }
				if turns == 0 {
					d.workerHeights[i] <- workerHeight + 1
				}
				startY := i * (workerHeight + 1)
				for yd := -1; yd <= workerHeight+1; yd++ {
					y := (startY + yd + p.imageHeight) % p.imageHeight
					// if turns == 0 {
					// 	fmt.Println("SENDING worker:", i, ", row:", y)
					// }
					for x := 0; x < p.imageWidth; x++ {
						d.workerVals[i] <- world[y][x]
					}
				}
			}
			for i := 0; i < p.threads-bigWorkers; i++ {
				// if turns == 0 {
				// 	fmt.Println("Dist sending small to worker", i+bigWorkers)
				// }
				if turns == 0 {
					d.workerHeights[i+bigWorkers] <- workerHeight
				}
				startY := (bigWorkers * (workerHeight + 1)) + i*workerHeight
				for yd := -1; yd <= workerHeight; yd++ {
					y := (startY + yd + p.imageHeight) % p.imageHeight
					// if turns == 0 {
					// 	fmt.Println("SENDING worker:", i+bigWorkers, ", row:", y)
					// }
					for x := 0; x < p.imageWidth; x++ {
						d.workerVals[i+bigWorkers] <- world[y][x]
					}
				}
			}

			//RECEIVE WORLD FROM WORKERS
			y = 0

			for i := 0; i < bigWorkers; i++ {
				// fmt.Println("Dist receiving big from worker", i)
				for ; y < (i+1)*(workerHeight+1); y++ {
					// fmt.Println("RECEIVING worker:", i, ", row:", y)
					for x := 0; x < p.imageWidth; x++ {
						tempWorld[y][x] = <-d.workerVals[i]
					}
				}
			}
			for i := 0; i < p.threads-bigWorkers; i++ {
				// fmt.Println("Dist receiving small from worker", i+bigWorkers)
				for ; y < (bigWorkers*(workerHeight+1))+(i+1)*workerHeight; y++ {
					// fmt.Println("RECEIVING worker:", i+bigWorkers, ", row:", y)
					for x := 0; x < p.imageWidth; x++ {
						tempWorld[y][x] = <-d.workerVals[i+bigWorkers]
					}
				}
			}

			//RECONSTRUCT WORLD
			world = tempWorld
		}

	}
	// fmt.Println(turns)
	// printGrid(world)

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

/* func aliveTimer(c chan int, i int) {

	for {
		time.Sleep(2 * time.Second)
		c <- i
		printalive := <-c
		fmt.Printf("%v\n", printalive)

	}
} */

func generatePGM(p golParams, d distributorChans, world [][]uint8) {
	d.io.command <- ioOutput
	d.io.filename <- strings.Join([]string{strconv.Itoa(p.imageWidth), strconv.Itoa(p.imageHeight)}, "x") + "_" + strconv.Itoa(p.turns)

	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			d.io.outputVal <- world[y][x]
		}
	}
}

func worker(p golParams, val chan uint8, heightIn chan int, num int) {
	var height = <-heightIn

	// Create the 2D slice to store the section of the world.
	world := make([][]byte, height+2)
	for i := range world {
		world[i] = make([]byte, p.imageWidth)
	}

	for turns := 0; turns < p.turns; turns++ {

		// Receive the section of the image byte by byte, in rows.
		for y := 0; y < height+2; y++ {
			for x := 0; x < p.imageWidth; x++ {
				world[y][x] = <-val
			}
		}

		tempWorld := make([][]byte, height+2)
		for i := range tempWorld {
			tempWorld[i] = make([]byte, p.imageWidth)
		}

		for y := 1; y <= height; y++ {
			for x := 0; x < p.imageWidth; x++ {

				alive := 0
				dead := 0

				for y1 := y - 1; y1 <= y+1; y1++ {
					for j := -1; j < 2; j++ {
						x1 := x + j
						if x1 == -1 {
							x1 = p.imageWidth - 1
						} else if x1 == p.imageWidth {
							x1 = 0
						}

						if x != x1 || y != y1 {
							if world[y1][x1] == 0 {
								dead++
							} else {
								alive++
							}
						}
					}
				}

				if world[y][x] == 0xFF {
					if alive < 2 || alive > 3 {
						tempWorld[y][x] = 0
					} else {
						tempWorld[y][x] = 0xFF
					}
				} else if world[y][x] == 0 && alive == 3 {
					tempWorld[y][x] = 0xFF
				}
			}
		}

		for y := 1; y <= height; y++ {
			for x := 0; x < p.imageWidth; x++ {
				val <- tempWorld[y][x]
			}
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
