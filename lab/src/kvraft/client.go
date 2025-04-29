package kvraft

import (
	"sync"

	"6.824/labrpc"
)

type Clerk struct {
	servers []*labrpc.ClientEnd
	// You will have to modify this struct.
	mu        sync.Mutex
	leaderIdx int
}

func MakeClerk(servers []*labrpc.ClientEnd) *Clerk {
	ck := new(Clerk)
	ck.servers = servers
	// You'll have to add code here.
	ck.leaderIdx = 0
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
	// You will have to modify this function.
	for {
		ck.mu.Lock()
		leaderIdx := ck.leaderIdx
		ck.mu.Unlock()

		reply := GetReply{}
		ok := ck.servers[leaderIdx].Call("KVServer.Get", &GetArgs{Key: key}, &reply)
		if ok {
			return reply.Value
		}
		switch reply.Err {
		case ErrNoKey:
			return ""
		case ErrWrongLeader:
			ck.mu.Lock()
			ck.leaderIdx = reply.LeaderIdx
			ck.mu.Unlock()
		default: // timeout. rotate server to another
			ck.mu.Lock()
			ck.leaderIdx = (ck.leaderIdx + 1) % len(ck.servers)
			ck.mu.Unlock()
		}
	}
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
	for {
		ck.mu.Lock()
		leaderIdx := ck.leaderIdx
		ck.mu.Unlock()

		reply := PutAppendReply{}
		ok := ck.servers[leaderIdx].Call("KVServer.PutAppend",
			&PutAppendArgs{
				Key:   key,
				Value: value,
				Op:    op,
			},
			&reply)
		if ok {
			return
		}
		switch reply.Err {
		case ErrWrongLeader:
			ck.mu.Lock()
			ck.leaderIdx = reply.LeaderIdx
			ck.mu.Unlock()
		default: // timeout. rotate server to another
			ck.mu.Lock()
			ck.leaderIdx = (ck.leaderIdx + 1) % len(ck.servers)
			ck.mu.Unlock()
		}
	}

}

func (ck *Clerk) Put(key string, value string) {
	ck.PutAppend(key, value, "Put")
}
func (ck *Clerk) Append(key string, value string) {
	ck.PutAppend(key, value, "Append")
}
