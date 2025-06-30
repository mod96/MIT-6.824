package kvraft

import (
	"sync"
	"time"

	"6.824/labrpc"
)

const (
	// RPC timeout for client requests
	RPCTimeout = 3000 * time.Millisecond
	// Delay when no leader is available
	NoLeaderDelay = 500 * time.Millisecond
)

type Clerk struct {
	servers []*labrpc.ClientEnd
	// You will have to modify this struct.
	mu           sync.Mutex
	leaderSrvIdx int // index of the server we believe is the leader
}

// MakeClerk creates a new Clerk instance to interact with the KV service
func MakeClerk(servers []*labrpc.ClientEnd) *Clerk {
	ck := new(Clerk)
	ck.servers = servers
	// You'll have to add code here.
	ck.leaderSrvIdx = 0 // start with the first server
	return ck
}

// fetch the current value for a key.
// returns "" if the key does not exist.
// keeps trying forever in the face of all other errors.
//
// If the Clerk sends an RPC to the wrong kvserver, or if it cannot reach the kvserver, the Clerk should re-try by sending to a different kvserver.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("KVServer.Get", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
func (ck *Clerk) Get(key string) string {
	if !validateKey(key) {
		DPrintf(dError, "Get: invalid key")
		return ""
	}

	DPrintf(dClerk, "get %s", key)
	// You will have to modify this function.
	ck.mu.Lock() // this is for simplicity. need modification for performance
	defer ck.mu.Unlock()
	reply := GetReply{}
	ck.rpcCallWithRetry("KVServer.Get", &GetArgs{Key: key}, &reply)
	if reply.GetErr() == ErrNoKey {
		return ""
	}
	return reply.Value
}

// shared by Put and Append.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("KVServer.PutAppend", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
func (ck *Clerk) PutAppend(key string, value string, op string) {
	if !validateKey(key) {
		DPrintf(dError, "PutAppend: invalid key")
		return
	}

	// You will have to modify this function.
	ck.mu.Lock()
	defer ck.mu.Unlock()
	reply := PutAppendReply{}
	ck.rpcCallWithRetry("KVServer.PutAppend",
		&PutAppendArgs{
			Key:   key,
			Value: value,
			Op:    op,
			ReqID: GenReqId(),
		},
		&reply)
}

func (ck *Clerk) Put(key string, value string) {
	DPrintf(dClerk, "put %s %s", key, value)
	ck.PutAppend(key, value, string(PutOp))
}
func (ck *Clerk) Append(key string, value string) {
	DPrintf(dClerk, "append %s %s", key, value)
	ck.PutAppend(key, value, string(AppendOp))
}

// for both, we need to find correct leader and retry while rpc succeeds
func (ck *Clerk) rpcCallWithRetry(svcMeth string, args interface{}, reply Reply) bool {
	// lock is held outside
	for {
		DPrintf(dClerk, "rpcCallWithRetry %s to srvIdx %d with args %v\n", svcMeth, ck.getCurrentServer(), args)

		reply.Clear() // clear reply before sending (for labgob compatibility)

		// send the RPC with timeout
		ok := ck.performRPCWithTimeout(svcMeth, args, reply)

		if ok {
			// RPC succeeded, handle the response
			if ck.handleErrorResponse(reply.GetErr()) {
				return true // operation completed successfully or with valid error
			}
			// else retry with different server
		} else {
			// RPC failed (timeout or network error), try next server
			DPrintf(dClerk, "rpcCallWithRetry %s failed on srvIdx %d\n", svcMeth, ck.getCurrentServer())
			ck.nextServer()
		}
	}
}

// performRPCWithTimeout executes an RPC call with timeout
func (ck *Clerk) performRPCWithTimeout(svcMeth string, args interface{}, reply Reply) bool {
	done := make(chan bool, 1)
	go func() {
		done <- ck.servers[ck.getCurrentServer()].Call(svcMeth, args, reply)
	}()
	select {
	case ok := <-done:
		return ok
	case <-time.After(RPCTimeout):
		return false // timeout
	}
}

// handleErrorResponse processes error responses and updates server selection
func (ck *Clerk) handleErrorResponse(err Err) bool {
	switch err {
	case ErrWrongLeader, ErrTimeout:
		ck.nextServer()
		return false // retry
	case ErrNoKey: // Get NoKey - this is a valid response
		return true
	case ErrNoLeader: // Leader is not elected yet
		time.Sleep(NoLeaderDelay)
		ck.nextServer()
		return false // retry
	case ErrBadRequest: // Invalid request
		return true // don't retry bad requests
	case OK:
		return true
	default:
		DPrintf(dError, "unexpected error: %s", err)
		return true // treat unknown errors as final
	}
}

// nextServer moves to the next server in round-robin fashion
func (ck *Clerk) nextServer() {
	ck.leaderSrvIdx = (ck.leaderSrvIdx + 1) % len(ck.servers)
}

// getCurrentServer returns the current server index
func (ck *Clerk) getCurrentServer() int {
	return ck.leaderSrvIdx
}

// validateKey checks if a key is valid
func validateKey(key string) bool {
	return key != ""
}
