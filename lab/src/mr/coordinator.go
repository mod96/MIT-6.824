package mr

import "log"
import "net"
import "os"
import "net/rpc"
import "net/http"
import "sync"

type FileState struct {
	processing bool
	done bool
	tried int
	fileNumber int
}


type Coordinator struct {
	// Your definitions here.
	nReduce int
	mu sync.Mutex
	filenames map[string]FileState
	mapStateDone bool
	incFileNumber int
}

// Your code here -- RPC handlers for the worker to call.

func (c *Coordinator) GetMapTask(args *GetMapTaskArgs, reply *GetMapTaskReply) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	reply.NReduce = c.nReduce

	c.mapStateDone = true
	for _, state := range c.filenames {
		if !state.done {
			c.mapStateDone = false
			break
		}
	}
	if c.mapStateDone {
        reply.Filename = ""
		return nil
	}
	for name, state := range c.filenames {
		if !state.processing && !state.done && state.tried < 5 {
			state.processing = true
			state.tried++
			state.fileNumber = c.incFileNumber
			c.incFileNumber++
			c.filenames[name] = state
			reply.Filename = name
			reply.FileNumber = state.fileNumber
			log.Printf("GetMapTask: %s\n", name)
            return nil
        }
	}
	reply.Filename = "-"
	return nil
}

func (c *Coordinator) MarkMapTaskDone(args *MarkMapTaskDoneArgs, reply *MarkMapTaskDoneReply) error {
	c.mu.Lock()
    defer c.mu.Unlock()
	filename := args.Filename
	log.Printf("MarkMapTaskDone: %s\n", filename)
	if entry, ok := c.filenames[filename]; ok {
		entry.done = true
		entry.processing = false
		c.filenames[filename] = entry
	}
    return nil
}

//
// an example RPC handler.
//
// the RPC argument and reply types are defined in rpc.go.
//
func (c *Coordinator) Example(args *ExampleArgs, reply *ExampleReply) error {
	reply.Y = args.X + 1
	return nil
}


//
// start a thread that listens for RPCs from worker.go
//
func (c *Coordinator) server() {
	rpc.Register(c)
	rpc.HandleHTTP()
	//l, e := net.Listen("tcp", ":1234")
	sockname := coordinatorSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

//
// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
//
func (c *Coordinator) Done() bool {
	ret := false

	// Your code here.


	return ret
}

//
// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
//
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	c := Coordinator{}
	c.nReduce = nReduce
	c.filenames = make(map[string]FileState, 0)
	c.mapStateDone = false
	c.incFileNumber = 0
	// Your code here.
	for _, filename := range files {
		c.filenames[filename] = FileState{processing: false, done: false, tried: 0}
	}
	
	// For this lab, have the coordinator wait for ten seconds; 
	// after that the coordinator should assume the worker has died
	
	


	c.server()
	return &c
}
