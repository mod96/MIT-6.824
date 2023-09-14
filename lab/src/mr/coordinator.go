package mr

import "log"
import "net"
import "os"
import "net/rpc"
import "net/http"
import "sync"
import "time"

type MapFileState struct {
	processing bool
	processingTime time.Time
	done bool
	tried int
	fileNumber int
}

type ReduceFiles struct {
	processing bool
	processingTime time.Time
	done bool
	tried int
	fileNames []string
}


type Coordinator struct {
	// Your definitions here.
	nReduce int
	mu sync.Mutex
	// map task related information
	mapFileNames map[string]MapFileState
	mapStateDone bool
	incFileNumber int
	// reduce task related information
	reduceFilenames map[int]ReduceFiles
	reduceStateDone bool
}

// Map related RPC handlers

func (c *Coordinator) GetMapTask(args *GetMapTaskArgs, reply *GetMapTaskReply) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	reply.NReduce = c.nReduce

	c.mapStateDone = true
	for _, state := range c.mapFileNames {
		if !state.done {
			c.mapStateDone = false
			break
		}
	}
	if c.mapStateDone {
        reply.Filename = ""
		return nil
	}
	for name, state := range c.mapFileNames {
		if !state.processing && !state.done {
			if state.tried < 5 {
				state.processing = true
				state.processingTime = time.Now()
				state.tried++
				state.fileNumber = c.incFileNumber
				c.incFileNumber++
				c.mapFileNames[name] = state
				reply.Filename = name
				reply.FileNumber = state.fileNumber
				if DebugPrint {
					log.Printf("Coordinator - GetMapTask: %s\n", name)
				}
				return nil
			} else {
				state.done = true
				c.mapFileNames[name] = state
			}
        }
	}
	reply.Filename = "-"
	return nil
}

func (c *Coordinator) MarkMapTaskDone(args *MarkMapTaskDoneArgs, reply *MarkMapTaskDoneReply) error {
	c.mu.Lock()
    defer c.mu.Unlock()
	filename := args.Filename
	if entry, ok := c.mapFileNames[filename]; ok {
		if entry.done {
			if DebugPrint {
				log.Printf("Coordinator - MarkMapTaskDone Ignored For Already Done: %s\n", filename)
			}
			return nil
		}
		if DebugPrint {
			log.Printf("Coordinator - MarkMapTaskDone: %s\n", filename)
		}
		entry.done = true
		entry.processing = false
		c.mapFileNames[filename] = entry

		// save filenames to reduceFilenames mapping
		reduceFileNameMapping := args.ReduceFileNameMapping
		for reduceKey, reduceFilename := range reduceFileNameMapping {
			if entry, ok := c.reduceFilenames[reduceKey]; ok {
				entry.fileNames = append(entry.fileNames, reduceFilename)
				c.reduceFilenames[reduceKey] = entry
			} else {
				c.reduceFilenames[reduceKey] = ReduceFiles{false, time.Time{}, false, 0, []string{reduceFilename}}
			}
		}
	}
    return nil
}

// Reduce related RPC handlers.

func (c *Coordinator) GetReduceTask(args *GetReduceTaskArgs, reply *GetReduceTaskReply) error {
	c.mu.Lock()
    defer c.mu.Unlock()

    c.reduceStateDone = true
    for _, state := range c.reduceFilenames {
        if !state.done {
            c.reduceStateDone = false
            break
        }
    }
    if c.reduceStateDone {
        reply.Filenames = nil
        return nil
    }
	for key, state := range c.reduceFilenames {
		if !state.processing &&!state.done {
            if state.tried < 5 {
                state.processing = true
                state.processingTime = time.Now()
                state.tried++
                c.reduceFilenames[key] = state
                reply.Filenames = state.fileNames
				reply.ReduceKey = key
				if DebugPrint {
                	log.Printf("[Coordinator] GetReduceTask: %d\n", key)
				}
                return nil
            } else {
                state.done = true
                c.reduceFilenames[key] = state
            }
        }
	}
	reply.Filenames = []string{"-"}
	return nil
}

func (c *Coordinator) MarkReduceTaskDone(args *MarkReduceTaskDoneArgs, reply *MarkReduceTaskDoneReply) error {
	c.mu.Lock()
    defer c.mu.Unlock()
    key := args.ReduceKey
    if entry, ok := c.reduceFilenames[key]; ok {
		if entry.done {
			if DebugPrint {
            	log.Printf("[Coordinator] MarkReduceTaskDone Ignored For Already Done: %d\n", key)
			}
            return nil
        }
		if DebugPrint {
			log.Printf("[Coordinator] MarkReduceTaskDone: %d\n", key)
		}
        entry.done = true
        entry.processing = false
        c.reduceFilenames[key] = entry
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
	// For this lab, have the coordinator wait for ten seconds; 
	// after that the coordinator should assume the worker has died
	c.mu.Lock()
    defer c.mu.Unlock()
	now := time.Now()
    for name, state := range c.mapFileNames {
        if !state.done && state.processingTime.Before(now.Add(-10 * time.Second)) {
            state.processing = false
			c.mapFileNames[name] = state
        }
    }
	for reduceKey, state := range c.reduceFilenames {
		if !state.done && state.processingTime.Before(now.Add(-10 * time.Second)) {
            state.processing = false
            c.reduceFilenames[reduceKey] = state
        }
	}
	if (c.mapStateDone && c.reduceStateDone && DebugPrint) {
		log.Printf("Coordinator Done\n")
    }
	return c.mapStateDone && c.reduceStateDone
}

//
// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
//
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	c := Coordinator{}
	c.nReduce = nReduce
	c.mapFileNames = make(map[string]MapFileState, 0)
	c.mapStateDone = false
	c.incFileNumber = 0
	c.reduceFilenames = make(map[int]ReduceFiles, 0)
	c.reduceStateDone = false
	// Your code here.
	for _, filename := range files {
		c.mapFileNames[filename] = MapFileState{processing: false, processingTime: time.Time{}, done: false, tried: 0, fileNumber: -1}
	}

	c.server()
	return &c
}
