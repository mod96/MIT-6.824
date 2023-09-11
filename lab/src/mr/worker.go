package mr

import "fmt"
import "io/ioutil"
import "log"
import "os"
import "time"
import "net/rpc"
import "hash/fnv"
import "strings"
import "sort"

//
// Map functions return a slice of KeyValue.
//
type KeyValue struct {
	Key   string
	Value string
}
// for sorting by key.
type ByKey []KeyValue

// for sorting by key.
func (a ByKey) Len() int           { return len(a) }
func (a ByKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKey) Less(i, j int) bool { return a[i].Key < a[j].Key }

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
	DoReduceTasks(reducef)

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
			reduceFileNameMapping := make(map[int]string)
			for i, entry := range buffer {
				intermediateFileName := fmt.Sprintf("intermediate-%d-%d", fileNumber, i)
				reduceFileNameMapping[i] = intermediateFileName
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
			args2 := MarkMapTaskDoneArgs{filename, reduceFileNameMapping}
			reply2 := MarkMapTaskDoneReply{}
			call("Coordinator.MarkMapTaskDone", &args2, &reply2)
		} else {
			log.Printf("GetMapTask call failed!\n")
		}
	}
}

func DoReduceTasks(reducef func(string, []string) string) {
	for {
		args := GetReduceTaskArgs{}
        reply := GetReduceTaskReply{}
        ok := call("Coordinator.GetReduceTask", &args, &reply)
		if ok {
			filenames := reply.Filenames
            reduceKey := reply.ReduceKey
            log.Printf("GetReduceTask: reduceKey=%d\n", reduceKey)
            // IF coordinator returned nothing, then go to next step
            if filenames == nil {
                return
            }
            // IF something else is working. reduce stage not finished
            if len(filenames) == 0 {
                time.Sleep(time.Second)
                continue
            }
            // IF coordinator returned something. read files and reduce stage
			intermediate := []KeyValue{}
			for _, filename := range filenames {
				file, err := os.Open(filename)
				if err!= nil {
					log.Fatalf("cannot open %v", filename)
				}
				content, err := ioutil.ReadAll(file)
				if err!= nil {
					log.Fatalf("cannot read %v", filename)
				}
				file.Close()
				for _, line := range strings.Split(string(content), "\n") {
                    kv := strings.Split(line, " ")
                    intermediate = append(intermediate, KeyValue{kv[0], kv[1]})
                }
			}
			sort.Sort(ByKey(intermediate))

			// do reduce function
			oname := fmt.Sprintf("mr-out-%d", reduceKey)
			ofile, _ := os.Create(oname)
            //
			// call Reduce on each distinct key in intermediate[],
			// and print the result to mr-out-0.
			//
			i := 0
			for i < len(intermediate) {
				j := i + 1
				for j < len(intermediate) && intermediate[j].Key == intermediate[i].Key {
					j++
				}
				values := []string{}
				for k := i; k < j; k++ {
					values = append(values, intermediate[k].Value)
				}
				output := reducef(intermediate[i].Key, values)

				// this is the correct format for each line of Reduce output.
				fmt.Fprintf(ofile, "%v %v\n", intermediate[i].Key, output)

				i = j
			}
			ofile.Close()
			// send to coordinator that reduce task is done
			args2 := MarkReduceTaskDoneArgs{reduceKey}
            reply2 := MarkReduceTaskDoneReply{}
            call("Coordinator.MarkReduceTaskDone", &args2, &reply2)
		} else {
			log.Printf("GetReduceTask call failed!\n")
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
