package main

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
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
				fmt.Println("Alive cell at", x, y)
				world[y][x] = val
			}
		}
	}

	workerHeight := p.imageHeight / p.threads
	bigWorkers := p.imageHeight % p.threads

	pause := false
	terminate := false

	// 2b - Print alive cells every 2 seconds
	ticker := time.NewTicker(2 * time.Second)
	go func() {
		for {
			<-ticker.C
			if pause == false {
				totalAlive := 0
				for y := 0; y < p.imageHeight; y++ {
					for x := 0; x < p.imageWidth; x++ {
						if world[y][x] == 0xFF {
							totalAlive++
						}
					}
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
			//STAGE 5!!!

			var wg sync.WaitGroup

			startY := 0
			workerHeight++
			for i := 0; i < p.threads; i++ {
				if i == bigWorkers {
					workerHeight--
				}
				//Save halos
				top, bottom := make([]uint8, p.imageWidth), make([]uint8, p.imageWidth)
				copy(top, world[(startY-1+p.imageHeight)%p.imageHeight])
				copy(bottom, world[(startY+workerHeight+p.imageHeight)%p.imageHeight])
				wg.Add(1)
				go func(startY, height int, topHalo, bottomHalo []uint8) {

					//Calculations
					numAlive := 0

					for y := startY; y < startY+height; y++ {
						for x := 0; x < p.imageWidth; x++ {

							//Count number of alive neighbours
							alive := 0
							for y1 := y - 1; y1 <= y+1; y1++ {
								for x1 := x - 1; x1 <= x+1; x1++ {
									if y1 == startY-1 {
										if topHalo[(x1+p.imageWidth)%p.imageWidth] == 0xFF {
											alive++
										}
									} else if y1 == startY+height {
										if bottomHalo[(x1+p.imageWidth)%p.imageWidth] == 0xFF {
											alive++
										}
									} else if x != x1 || y != y1 {
										if world[y1][(x1+p.imageWidth)%p.imageWidth] == 0xFF {
											alive++
										}
									}
								}
							}

							//Decide whether cell lives or dies
							if world[y][x] == 0xFF {
								if alive < 2 || alive > 3 {
									world[y][x] = 0
								} else {
									world[y][x] = 0xFF
									numAlive++
								}
							} else if world[y][x] == 0 && alive == 3 {
								world[y][x] = 0xFF
								numAlive++
							}

						}
					}
					wg.Done()
				}(startY, workerHeight, top, bottom)
				startY += workerHeight
				wg.Wait()
			}
			turns++
		}

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
