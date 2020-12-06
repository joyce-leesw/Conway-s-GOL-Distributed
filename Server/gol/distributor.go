package main

import (

	//"sync"
	//"time"
	"fmt"
	"net"
	"os"
	"sync"

	"log"
	"net/http"
	"net/rpc"
	//"uk.ac.bris.cs/gameoflife/util"
)

// type distributorChannels struct {
// 	events    chan<- Event
// 	ioCommand chan<- ioCommand
// 	ioIdle    <-chan bool

// 	filename chan<- string
// 	outputQ  chan<- uint8
// 	inputQ   <-chan uint8
// }
type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

type ServerDistributorStruct struct {
	P Params
	//C distributorChannels
	//keyPresses <-chan rune
	ControllerFlag chan int // remooooooooooooooooooooovvveeee
	InputWorld     [][]uint8
}

type API int

var world [][]uint8
var turn int
var mutex = &sync.Mutex{}

//var controllerFlag chan int
var controllerFlag = make(chan int, 2)

//var bufferedWorld = make(chan int, 1)

type Item struct {
	PWorld [][]uint8
}

type ItemW struct {
	SWorld  [][]uint8
	TurnCur int
}

type Cf struct {
	Flag int
}

// Currently alive cells and current turns
type Ae struct {
	Alive   int
	CurTurn int
}

func (a *API) CFput(num Cf, reply *Cf) error {
	var cFlag int
	cFlag = num.Flag
	controllerFlag <- cFlag
	*reply = Cf{turn}
	return nil
}

func (a *API) GetWorld(dead Cf, reply *ItemW) error {
	mutex.Lock()
	*reply = ItemW{world, turn}
	mutex.Unlock()
	return nil
}

func (a *API) Alivecount(num Ae, reply *Ae) error {
	mutex.Lock()
	t := calculateAliveCells(world)
	*reply = Ae{t, turn}
	mutex.Unlock()
	return nil
}

// type Cell struct {
// 	X, Y int
// }
func (a *API) KillProg(kill Cf, reply *Cf) error {
	os.Exit(0)
	return nil
}

// distributor divides the work between workers and interacts with other goroutines.
func (a *API) ServerDistributor(req ServerDistributorStruct, reply *Item) error {

	//var state State
	StrSerList := []string{"localhost:8030", "localhost:8040", "localhost:8050", "localhost:8060"}
	//controllerFlag := make(chan int, 2)
	var SubSerList []*rpc.Client
	for _, s := range StrSerList {
		client, err := rpc.DialHTTP("tcp", s) //"127.0.0.1:8030"  //rpc.DialHTTP("tcp", "localhost:8080")
		if err != nil {
			log.Fatal("Connection error: ", err)
			fmt.Println("con fail :(")
		} else {
			SubSerList = append(SubSerList, client)
		}

	}
	SubSerCount := len(SubSerList)

	sliceOfCh := make([]chan [][]uint8, SubSerCount) // make a separate channel for each divided piece of the game board

	world = req.InputWorld

	for turnf := 0; turnf < req.P.Turns; turnf++ {

		baseLines := req.P.ImageHeight / SubSerCount
		slackLines := req.P.ImageHeight % SubSerCount // remainder
		workLines := make([]int, SubSerCount)

		for i := 0; i < SubSerCount; i++ { // create an array with the minimum amount of lines to work on
			workLines[i] = baseLines
		}

		for i := 0; i < slackLines; i++ { // adds the remainder to the array
			workLines[i]++
		}

		for i := 0; i < SubSerCount; i++ { //call a worker for each section we are splitting the board into
			sliceOfCh[i] = make(chan [][]uint8)

			go worker(workLines, i, world, sliceOfCh[i], req.P, SubSerList)
		}

		var newData [][]uint8

		for i := 0; i < SubSerCount; i++ { // take our updated parts and put them back togther
			slice := <-sliceOfCh[i]
			newData = append(newData, slice...)
		}

		mutex.Lock()
		world = newData //update the board state
		turn++
		mutex.Unlock()
		//fmt.Println("")

		// select {
		// case <-bufferedWorld: //t :=
		// 	bufferedWorld <- calculateAliveCells(req.P, newData)
		// 	//fmt.Println(t)

		// default:
		// 	bufferedWorld <- calculateAliveCells(req.P, newData)
		// }

		//bufferedWorld <- newData

		var keyFlag int
		//fmt.Println("does this print 1?")

		controllerFlag <- 4
		//fmt.Println("does this print 1?")

		keyFlag = <-controllerFlag
		fmt.Println(keyFlag)

		if keyFlag == 2 { // when q is pressed, quit turn
			//state = 2
			//c.events <- StateChange{turn, state}
			<-controllerFlag // remove 4 from the buffer
			break
		}
		if keyFlag == 0 { // when p is pressed, pause turn
			//state = 0
			<-controllerFlag
			//c.events <- StateChange{turn, state}
			for {
				keyFlag = <-controllerFlag
				if keyFlag == 0 { // when p is pressed again, resume
					//state = 1
					fmt.Println("Continuing")
					//c.events <- StateChange{turn, state}
					break
				}
			}
		}
		if keyFlag == 5 {
			<-controllerFlag // remove 4 from the buffer
			for _, ks := range SubSerList {
				var np Cf
				ks.Call("API.KillProg", Cf{0}, &np)
			}

			break

		}
		// if keyFlag == 1 { // when s is pressed, print current turn
		// 	outName := fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, turn)
		// }

		// c.ioCommand <- ioOutput
		// c.filename <- outName

		// for i := 0; i < (p.ImageHeight); i++ {
		// 	for j := 0; j < (p.ImageHeight); j++ {
		// 		c.outputQ <- world[i][j]
		// 	}
		// }
		// //c.events <- ImageOutputComplete{turn, outName}
		// <-req.controllerFlag

		//}
	}

	*reply = Item{PWorld: world}
	//os.Exit(0)

	return nil

}

// func numCalculateAliveCells(world [][]uint8) int {
// 	aliveCells := 0
// 	for y := 0; y < len(world); y++ {
// 		for x := 0; x < len(world); x++ {
// 			if world[y][x] == 255 {
// 				aliveCells++
// 			}
// 		}
// 	}
// 	return aliveCells
// }

func calculateAliveCells(world [][]uint8) int {
	aliveCells := 0
	for y := 0; y < len(world); y++ {
		for x := 0; x < len(world); x++ {
			if world[y][x] == 255 {
				aliveCells++
			}
		}
	}
	return aliveCells
}

func worker(lines []int, sliceNum int, world [][]uint8, sliceOfChi chan<- [][]uint8, p Params, SubSerList []*rpc.Client) {
	var worldGo [][]uint8
	var newData [][]uint8

	compLines := 0

	SubSerCount := len(SubSerList)

	for i := 0; i < sliceNum; i++ {
		compLines = compLines + lines[i]
	}

	if sliceNum == 0 { // is this the first slice of the GOL board
		newData = append(newData, world[p.ImageHeight-1]) // add last line to start
		if SubSerCount == 1 {                             //is this the only slice
			worldGo = world
			worldGo = append(worldGo, world[0]) // add first line to the end if we have 1 worker
		} else {
			worldGo = world[:lines[sliceNum]+1]
		}

		worldGo = append(newData, worldGo...)

	} else if sliceNum == SubSerCount-1 { //is this neither the first or last slice of the GOL board
		worldGo = world[compLines-1:]
		worldGo = append(worldGo, world[0])
	} else { //is this the last slice of the GOL board
		worldGo = world[compLines-1 : compLines+lines[sliceNum]+1]
	}

	var subPart Item
	sendWorld := ServerDistributorStruct{p, controllerFlag, worldGo}
	// replace client with this
	fmt.Println("worldGo %d", len(worldGo))
	SubSerList[sliceNum].Call("API.SubServerDistributor", sendWorld, &subPart)
	part := subPart.PWorld
	fmt.Println("part %d", len(part))

	//part := calculateNextState(p, worldGo, compLines)

	sliceOfChi <- part[1 : len(part)-1]
}

// func calculateNextState(p Params, world [][]uint8, compLines int) [][]uint8 {

// 	var counter, nextX, lastX, nextY, lastY int //can I use a byte here instead
// 	newWS := make([][]uint8, (len(world) - 2))
// 	for i := 0; i < (len(world) - 2); i++ {
// 		newWS[i] = make([]uint8, p.ImageWidth)
// 	}

// 	tempW := world[1 : len(world)-1]

// 	for y, s := range tempW {

// 		for x, sl := range s {

// 			counter = 0

// 			//Set the nextX and lastX variables
// 			if x == len(s)-1 { //are we looking at the last element of the slice
// 				nextX = 0
// 				lastX = x - 1
// 			} else if x == 0 { //are we looking at the first element of the slice
// 				nextX = x + 1
// 				lastX = len(s) - 1
// 			} else { //we are looking at any element that is not the first of last element of a slice
// 				nextX = x + 1
// 				lastX = x - 1
// 			}

// 			//Set the nextY and lastY variables
// 			nextY = y + 2
// 			lastY = y

// 			if 255 == s[nextX] {
// 				counter++
// 			} //look E
// 			if 255 == s[lastX] {
// 				counter++
// 			} //look W

// 			if 255 == world[lastY][lastX] {
// 				counter++
// 			} //look NW
// 			if 255 == world[lastY][x] {
// 				counter++
// 			} //look N
// 			if 255 == world[lastY][nextX] {
// 				counter++
// 			} //look NE

// 			if 255 == world[nextY][lastX] {
// 				counter++
// 			} //look SW
// 			if 255 == world[nextY][x] {
// 				counter++
// 			} //look S
// 			if 255 == world[nextY][nextX] {
// 				counter++
// 			} //look SE

// 			//Live cells
// 			if sl == 255 {
// 				if counter < 2 || counter > 3 { //"any live cell with fewer than two or more than three live neighbours dies"
// 					newWS[y][x] = 0
// 					//c.events <- CellFlipped{turnf, util.Cell{X: x, Y: (y + compLines)}}

// 				} else { //"any live cell with two or three live neighbours is unaffected"
// 					newWS[y][x] = 255
// 				}

// 			}

// 			//Dead cell -- not MGS
// 			if sl == 0 {
// 				if counter == 3 { //"any dead cell with exactly three live neighbours becomes alive"
// 					newWS[y][x] = 255
// 					//c.events <- CellFlipped{turnf, util.Cell{X: x, Y: (y + compLines)}}

// 				} else {
// 					newWS[y][x] = 0 // Dead cells elsewise stay dead
// 				}

// 			}

// 		}

// 	}

// 	return newWS

// }

func main() {
	//controllerFlag := make(chan int, 2)

	var api = new(API)
	err := rpc.Register(api)
	if err != nil {
		log.Fatal("error registering the api thing", err)
	}

	rpc.HandleHTTP()
	listener, err := net.Listen("tcp", ":8080")

	if err != nil {
		log.Fatal("worry time listener error", err)
	}

	log.Printf("serving the client rpc on port %d", 8080)
	err = http.Serve(listener, nil)
	if err != nil {
		log.Fatal("error serving: ", err)
	}

}
