package kvraft

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
)

type logTopic string

const (
	dClerk  logTopic = "CLRK"
	dServer logTopic = "SERV"
	dError  logTopic = "ERRO"
	dInfo   logTopic = "INFO"
	dLog    logTopic = "LOG1"
	dTest   logTopic = "TEST"
	dTrace  logTopic = "TRCE"
	dWarn   logTopic = "WARN"
)

var debugStart time.Time
var debugVerbosity int

// Retrieve the verbosity level from an environment variable
func getVerbosity() int {
	v := os.Getenv("VERBOSE")
	level := 0
	if v != "" {
		var err error
		level, err = strconv.Atoi(v)
		if err != nil {
			log.Fatalf("Invalid verbosity %v", v)
		}
	}
	return level
}

func DInit() {
	debugVerbosity = getVerbosity()
	debugStart = time.Now()

	log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))
}

func DPrintf(topic logTopic, format string, a ...interface{}) (n int, err error) {
	if debugVerbosity >= 1 {
		time := time.Since(debugStart).Microseconds()
		time /= 100
		prefix := fmt.Sprintf("%06d %v ", time, string(topic))
		format = prefix + format
		log.Printf(format, a...)
	}
	return
}

// generate unique identifier for each request.
// this is used to deduplicate requests in the server.
// It is unique across all Clerk instances and all servers by
// using uuid and timestamp.
func GenReqId() string {
	return time.Now().Format("20060102150405.000") + "-" + uuid.New().String()
}
