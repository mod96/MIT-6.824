package kvraft

import (
	"sync"
	"time"

	"6.824/labrpc"
)

type Clerk struct {
	servers []*labrpc.ClientEnd
	// You will have to modify this struct.
	mu           sync.Mutex
	leaderSrvIdx int
}

func MakeClerk(servers []*labrpc.ClientEnd) *Clerk {
	ck := new(Clerk)
	ck.servers = servers
	// You'll have to add code here.
	ck.leaderSrvIdx = 0
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
	ck.PutAppend(key, value, "Put")
}
func (ck *Clerk) Append(key string, value string) {
	DPrintf(dClerk, "append %s %s", key, value)
	ck.PutAppend(key, value, "Append")
}

// for both, we need to find correct leader and retry while rpc succeeds
func (ck *Clerk) rpcCallWithRetry(svcMeth string, args interface{}, reply Reply) bool {
	// lock is held outside
	for {
		DPrintf(dClerk, "rpcCallWithRetry %s to srvIdx %d with args %v\n", svcMeth, ck.leaderSrvIdx, args)

		// DPrintf(dClerk, "calling %s with %v on server %d\n", svcMeth, args, leaderSrvIdx)
		reply.Clear() // clear reply before sending (for labgob compatibility)
		// send the RPC
		ok := func() bool {
			done := make(chan bool, 1)
			go func() {
				done <- ck.servers[ck.leaderSrvIdx].Call(svcMeth, args, reply)
			}()
			select {
			case ok := <-done:
				return ok
			case <-time.After(3000 * time.Millisecond):
				return false // timeout
			}
		}()
		if ok {
			// DPrintf(dClerk, "reply.Err: %s from srvIdx %d\n", reply.GetErr(), ck.leaderSrvIdx)
			switch reply.GetErr() {
			case ErrWrongLeader, ErrTimeout:
				ck.leaderSrvIdx = (ck.leaderSrvIdx + 1) % len(ck.servers)
			case ErrNoKey: // Get NoKey
				return true
			case ErrNoLeader: // Leader is not elected yet
				time.Sleep(500 * time.Millisecond)
				ck.leaderSrvIdx = (ck.leaderSrvIdx + 1) % len(ck.servers)
			case OK:
				return true
			default:
				panic("unexpected error")
			}
		} else {
			// DPrintf(dClerk, "rpcCallWithRetry %s failed on srvIdx %d\n", svcMeth, ck.leaderSrvIdx)
			// RPC failed, try next server
			ck.leaderSrvIdx = (ck.leaderSrvIdx + 1) % len(ck.servers)
		}
	}
}
