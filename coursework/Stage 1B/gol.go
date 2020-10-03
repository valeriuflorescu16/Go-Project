package main

import (
	"fmt"
	"strconv"
	"strings"
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
				fmt.Println("Alive cell at", x, y)
				world[y][x] = val
			}
		}
	}

	// Calculate the new state of Game of Life after the given number of turns.
	for turns := 0; turns < p.turns; turns++ {

		tempWorld := make([][]byte, p.imageHeight)
		for i := range tempWorld {
			tempWorld[i] = make([]byte, p.imageWidth)
		}

		//WORKER CODE
		workerHeight := p.imageHeight / p.threads

		// Send the section of the image to workers byte by byte, in rows.
		for i := 0; i < p.threads; i++ {
			startY := i * workerHeight
			for yd := -1; yd <= workerHeight; yd++ {
				y := (startY + yd + p.imageHeight) % p.imageHeight
				for x := 0; x < p.imageWidth; x++ {
					d.workerVals[i] <- world[(y+p.imageHeight)%p.imageHeight][x]
				}
			}
		}

		//RECEIVE WORLD FROM WORKERS
		for i := 0; i < p.threads; i++ {
			startY := i * workerHeight
			for y := startY; y < startY+workerHeight; y++ {
				for x := 0; x < p.imageWidth; x++ {
					tempWorld[y][x] = <-d.workerVals[i]
				}
			}
		}

		//RECONSTRUCT WORLD
		world = tempWorld
	}

	//Send world to pgm one byte at a time
	d.io.command <- ioOutput
	d.io.filename <- strings.Join([]string{strconv.Itoa(p.imageWidth), strconv.Itoa(p.imageHeight)}, "x") + "_" + strconv.Itoa(p.turns)

	for y := 0; y < p.imageHeight; y++ {
		for x := 0; x < p.imageWidth; x++ {
			d.io.outputVal <- world[y][x]
		}
	}

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

func worker(p golParams, val chan uint8, num int) {
	height := p.imageHeight / p.threads

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

		//Send tempWorld to distributor one byte at a time
		for y := 1; y <= height; y++ {
			for x := 0; x < p.imageWidth; x++ {
				val <- tempWorld[y][x]
			}
		}
	}
}

func printGrid(world [][]byte) {
	for y := 0; y < len(world); y++ {
		for x := 0; x < len(world[0]); x++ {
			fmt.Print(world[y][x])
			if x >= len(world[0])-1 {
				fmt.Println()
			}
		}
	}
}
