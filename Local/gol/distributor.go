package gol

import (
	"fmt"
	//"sync"

	"time"
	//"net"

	"log"
	"net/rpc"

	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events    chan<- Event
	ioCommand chan<- ioCommand
	ioIdle    <-chan bool

	filename chan<- string
	outputQ  chan<- uint8
	inputQ   <-chan uint8
}

type Item struct {
	PWorld [][]uint8
}

type ItemW struct {
	SWorld  [][]uint8
	TurnCur int
}

type Ae struct {
	Alive   int
	CurTurn int
}

type Cf struct {
	Flag int
}

type ServerDistributorStruct struct {
	P Params
	//C distributorChannels
	//keyPresses <-chan rune
	ControllerFlag <-chan int
	InputWorld     [][]uint8
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels, keyPresses <-chan rune) {

	ticker := time.NewTicker(2 * time.Second) // create a ticker for our alivecellscount event anon func
	done := make(chan bool)                   // so we can end the anon go function for allivecellscount

	var reply Item
	var state State
	state = 1
	//var req ServerDistributorStruct

	controllerFlag := make(chan int, 2)

	var turn int // undecalered value is zero

	c.events <- StateChange{turn, state}

	initialWorld := make([][]uint8, p.ImageHeight) //make empty board heightxwidth
	for i := 0; i < (p.ImageHeight); i++ {
		initialWorld[i] = make([]uint8, p.ImageWidth)
	}

	//open the pgm file to game of life
	fileName := fmt.Sprintf("%vx%v", p.ImageWidth, p.ImageHeight)

	//read the game of life and convert the pgm into a slice of slices
	c.ioCommand <- ioInput
	c.filename <- fileName

	for i := 0; i < (p.ImageHeight); i++ { //take the bytes we get from the inputQ channel and populate the empty board
		for j := 0; j < (p.ImageHeight); j++ {
			initialWorld[i][j] = <-c.inputQ
		}
	}

	// TODO: For all initially alive cells send a CellFlipped Event.
	for _, cellQ := range calculateAliveCells(p, initialWorld) {
		c.events <- CellFlipped{0, cellQ}
	}

	// TODO: Execute all turns of the Game of Life.
	//var state State

	world := initialWorld

	///
	///
	//call our rpc function
	client, err := rpc.DialHTTP("tcp", "localhost:8080")

	if err != nil {
		log.Fatal("Connection error: ", err)
	}
	//fmt.Println("does this print 1?")

	go func() {
		var answer Cf

		for {
			keypressed := <-keyPresses
			switch {
			case keypressed == 113: // when q is pressed
				client.Call("API.CFput", Cf{2}, &answer)
				state = 2
				//c.events <- StateChange{turn, state}

			case keypressed == 112: // when p is pressed
				client.Call("API.CFput", Cf{0}, &answer)
				state = 0
				c.events <- StateChange{answer.Flag, state}
				for {
					keypressed = <-keyPresses
					if keypressed == 112 { // when p is pressed again, resume
						state = 1
						fmt.Println("Continuing")
						c.events <- StateChange{answer.Flag, state}
						client.Call("API.CFput", Cf{0}, &answer)
						break
					}
				}
			case keypressed == 115: // when s is pressed
				var wt ItemW
				client.Call("API.GetWorld", Cf{0}, &wt)
				outName := fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, wt.TurnCur)

				c.ioCommand <- ioOutput
				c.filename <- outName

				for i := 0; i < (p.ImageHeight); i++ {
					for j := 0; j < (p.ImageHeight); j++ {
						c.outputQ <- wt.SWorld[i][j]
					}
				}
				c.events <- ImageOutputComplete{wt.TurnCur, outName}

			case keypressed == 107: // when k is pressed
				client.Call("API.CFput", Cf{5}, &answer)
				state = 2
				//c.events <- StateChange{turn, state}

			}

		}
	}()

	go func() { //anon function that reports alivecellcount every two seconds
		for {
			select {
			case <-ticker.C:
				var cellcount Ae

				client.Call("API.Alivecount", Ae{0, 0}, &cellcount)
				c.events <- AliveCellsCount{cellcount.CurTurn, cellcount.Alive}
			case <-done:
				return

			}
		}
	}()

	sendWorld := ServerDistributorStruct{p, controllerFlag, world}
	client.Call("API.ServerDistributor", sendWorld, &reply)
	//fmt.Println("does this print 2?")

	///
	///
	///

	// TODO: Send correct Events when required, e.g. CellFlipped, TurnComplete and FinalTurnComplete.
	//		 See event.go for a list of all events.

	var endturn Ae
	client.Call("API.Alivecount", Ae{0, 0}, &endturn)

	turn = endturn.CurTurn

	for _, cellQ := range calculateAliveCells(p, initialWorld) { //kill cells
		c.events <- CellFlipped{0, cellQ}
	}
	//fmt.Println("does this print 3?")
	world = reply.PWorld

	for _, cellQ := range calculateAliveCells(p, world) { //live cells
		c.events <- CellFlipped{0, cellQ}
	}
	c.events <- FinalTurnComplete{turn, calculateAliveCells(p, world)}
	c.events <- StateChange{turn, Quitting}

	ticker.Stop() //stop ticker
	done <- true  // send fl;ag to finish anon go routine for alivecellscount

	// output state of game as PGM after all turns completed
	outName := fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, turn)

	c.ioCommand <- ioOutput
	c.filename <- outName

	for i := 0; i < (p.ImageHeight); i++ {
		for j := 0; j < (p.ImageHeight); j++ {
			c.outputQ <- world[i][j]
		}
	}

	var np Cf
	client.Call("API.KillProg", Cf{0}, &np)

	c.events <- ImageOutputComplete{turn, outName}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	//c.events <- StateChange{turn, Quitting}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}

func calculateAliveCells(p Params, world [][]uint8) []util.Cell {
	aliveCells := []util.Cell{}
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			if world[y][x] == 255 {
				aliveCells = append(aliveCells, util.Cell{X: x, Y: y})
			}
		}
	}
	return aliveCells
}

// func controller(keyPresses <-chan rune) {
// 	var answer Cf

// 	for {
// 		keypressed := <-keyPresses
// 		switch {
// 		case keypressed == 113: // when q is pressed
// 			client.Call("API.CFput", Cf{2}, &answer)
// 		case keypressed == 112: // when p is pressed
// 			client.Call("API.CFput", Cf{0}, &answer)
// 		case keypressed == 115: // when s is pressed
// 			client.Call("API.CFput", Cf{1}, &answer)

// 		}

// 	}

// }
