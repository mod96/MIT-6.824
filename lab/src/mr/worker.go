package mr

import "fmt"
import "io/ioutil"
import "log"
import "os"
import "time"
import "net/rpc"
import "hash/fnv"


//
// Map functions return a slice of KeyValue.
//
type KeyValue struct {
	Key   string
	Value string
}

//
// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
//
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}


//
// main/mrworker.go calls this function.
//
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {

	// Your worker implementation here.

	// uncomment to send the Example RPC to the coordinator.
	DoMapTasks(mapf)

}

func DoMapTasks(mapf func(string, string) []KeyValue) {
	for {
		args := GetMapTaskArgs{}
		reply := GetMapTaskReply{}
		ok := call("Coordinator.GetMapTask", &args, &reply)
		if ok {
			filename := reply.Filename
			nReduce := reply.NReduce
			fileNumber := reply.FileNumber
			log.Printf("GetMapTask: filename=%s nReduce=%d\n", filename, nReduce)
			// IF coordinator returned nothing, then go to next step
			if filename == "" {
				return
			}
			// IF something else is working. map stage not finished
			if filename == "-" {
				time.Sleep(time.Second)
				continue
			}
			// IF coordinator returned something. read file
			file, err := os.Open(filename)
			if err != nil {
				log.Fatalf("cannot open %v", filename)
			}
			content, err := ioutil.ReadAll(file)
			if err != nil {
				log.Fatalf("cannot read %v", filename)
			}
			file.Close()
			// do map function
			kva := mapf(filename, string(content))
			// buffer result to slices of nReduce
			buffer := make(map[int][]KeyValue)
			for _, keyValue := range kva {
				hashKey := ihash(keyValue.Key) % nReduce
				buffer[hashKey] = append(buffer[hashKey], keyValue)
			}
			// write result to intermediate files
			for i, entry := range buffer {
				intermediateFileName := fmt.Sprintf("intermediate-%d-%d", fileNumber, i)
                file, err := os.OpenFile(intermediateFileName, os.O_CREATE|os.O_WRONLY, 0644)
                if err!= nil {
                    log.Fatalf("cannot open %v", intermediateFileName)
                }
				for _, keyValue := range entry {
					fmt.Fprintf(file, "%v %v\n", keyValue.Key, keyValue.Value)
				}
                file.Close()
			}
			// send to reduce stage
			args2 := MarkMapTaskDoneArgs{filename}
			reply2 := MarkMapTaskDoneReply{}
			call("Coordinator.MarkMapTaskDone", &args2, &reply2)
		} else {
			log.Printf("call failed!\n")
		}
	}
}

//
// example function to show how to make an RPC call to the coordinator.
//
// the RPC argument and reply types are defined in rpc.go.
//
func CallExample() {

	// declare an argument structure.
	args := ExampleArgs{}

	// fill in the argument(s).
	args.X = 99

	// declare a reply structure.
	reply := ExampleReply{}

	// send the RPC request, wait for the reply.
	// the "Coordinator.Example" tells the
	// receiving server that we'd like to call
	// the Example() method of struct Coordinator.
	ok := call("Coordinator.Example", &args, &reply)
	if ok {
		// reply.Y should be 100.
		fmt.Printf("reply.Y %v\n", reply.Y)
	} else {
		fmt.Printf("call failed!\n")
	}
}

//
// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
//
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := coordinatorSock()
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}
