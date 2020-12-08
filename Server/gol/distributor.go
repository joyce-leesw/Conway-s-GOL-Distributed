package main

import (
	"fmt"
	"net"
	"os"
	"sync"

	"log"
	"net/http"
	"net/rpc"
)

type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

type ServerDistributorStruct struct {
	P          Params
	InputWorld [][]uint8
	SubAddList []string
}

type API int

var world [][]uint8
var turn int
var mutex = &sync.Mutex{}

var controllerFlag = make(chan int, 2)

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

func (a *API) KillProg(kill Cf, reply *Cf) error {
	os.Exit(0)
	return nil
}

// distributor divides the work between workers and interacts with other goroutines.
func (a *API) ServerDistributor(req ServerDistributorStruct, reply *Item) error {

	StrSerList := req.SubAddList //[]string{"localhost:8030", "localhost:8040", "localhost:8050", "localhost:8060"}

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

		var keyFlag int

		controllerFlag <- 4

		keyFlag = <-controllerFlag
		fmt.Println(keyFlag)

		if keyFlag == 2 { // when q is pressed, quit turn
			<-controllerFlag // remove 4 from the buffer
			break
		}
		if keyFlag == 0 { // when p is pressed, pause turn
			<-controllerFlag
			for {
				keyFlag = <-controllerFlag
				if keyFlag == 0 { // when p is pressed again, resume
					fmt.Println("Continuing")
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
	}

	*reply = Item{PWorld: world}

	return nil

}

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
	sendWorld := ServerDistributorStruct{p, worldGo, nil}
	// replace client with this
	fmt.Println("worldGo %d", len(worldGo))
	SubSerList[sliceNum].Call("API.SubServerDistributor", sendWorld, &subPart)
	part := subPart.PWorld
	fmt.Println("part %d", len(part))

	sliceOfChi <- part[1 : len(part)-1]
}

func main() {

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
