package kvraft

import (
	"sync"
	"sync/atomic"
	"time"

	"6.824/labgob"
	"6.824/labrpc"
	"6.824/raft"
)

type Op struct {
	// Your definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
	OpName string
	Key    string
	Value  string
	ID     int // unique ID for the operation, used to deduplicate
}

const (
	NoOp     = "NoOp"
	PutOp    = "Put"
	AppendOp = "Append"
)

type KVServer struct {
	mu      sync.Mutex
	me      int
	rf      *raft.Raft
	applyCh chan raft.ApplyMsg
	dead    int32 // set by Kill()

	maxraftstate int // snapshot if log grows this big

	// Your definitions here.
	leaderInfo struct {
		leaderId int
		term     int
	}
	kvStore map[string]string // key-value store

	canReturn map[int]chan struct{} // map of OpIDs to channels to signal when the operation is done
	opGen     int64                 // used to generate unique Op IDs
}

// *********************************** Private server APIs for encapsulated calls

// generate new Op ID.
func (kv *KVServer) genOpID() int {
	atomic.AddInt64(&kv.opGen, 1)
	return int(kv.opGen)
}

// sendMsg sends a message to the raft server and waits for a reply.
func (kv *KVServer) sendMsg(op Op) bool {
	// lock is held outside.
	// send the message to raft
	// DPrintf(dServer, "%d, sendMsg: op=%v - H1", kv.me, op)
	kv.canReturn[op.ID] = make(chan struct{}) // create a channel to signal when the operation is done
	// send the operation to raft
	_, _, rfIsLeader := kv.rf.Start(op)
	// DPrintf(dServer, "%d, sendMsg: op=%v - H2", kv.me, op)
	if !rfIsLeader {
		panic("not leader")
	}
	// wait for chan close
	select {
	case <-kv.canReturn[op.ID]: // wait for the operation to be done
		DPrintf(dServer, "%d, sendMsg: op=%v done\n", kv.me, op)
		close(kv.canReturn[op.ID])
		delete(kv.canReturn, op.ID)
		return true
	case <-time.After(10 * time.Second): // timeout after 10 seconds
		DPrintf(dServer, "%d, sendMsg: op=%v timeout\n", kv.me, op)
		close(kv.canReturn[op.ID])
		delete(kv.canReturn, op.ID) // remove the channel from the map
		return false
	}
}

// lock is held outside.
// getleader tries to fetch leaderInfo from rf.
// if the leaderInfo is up to date, it returns true.
// if the leaderInfo is not up to date, it updates the leaderInfo,
// send no-ops to the leader, and returns true. while doing this,
// if timeout, it returns false.
func (kv *KVServer) getLeader() bool {
	// lock is held outside.
	rfTerm, _ := kv.rf.GetState()
	rfLeader := kv.rf.GetLeaderIdx()
	if rfLeader == -1 {
		DPrintf(dServer, "%d, getLeader: no leader\n", kv.me)
		return false
	}
	// DPrintf(dServer, "%d, getLeader: rfTerm=%d rfLeader=%d kvTerm=%d kvLeader=%d\n",
	// 	kv.me, rfTerm, rfLeader, kv.leaderInfo.term, kv.leaderInfo.leaderId)
	if kv.leaderInfo.term == rfTerm && kv.leaderInfo.leaderId == rfLeader {
		return true
	}
	// update leaderInfo
	DPrintf(dServer, "%d, getLeader: update leaderInfo to rfTerm=%d rfLeader=%d\n",
		kv.me, rfTerm, rfLeader)
	kv.leaderInfo.term = rfTerm
	kv.leaderInfo.leaderId = rfLeader
	// send no-ops if i am the leader
	if kv.leaderInfo.leaderId == kv.me {
		// send no-op to myself with a timeout
		return kv.sendMsg(Op{OpName: NoOp, ID: kv.genOpID()})
	}
	return true
}

// *********************************** Public server APIs for client calls

// Get is the RPC handler for the Get method.
func (kv *KVServer) Get(args *GetArgs, reply *GetReply) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	if !kv.getLeader() {
		reply.Err = ErrNoLeader
		return
	}
	DPrintf(dServer, "%d, Get: leaderInfo=%v\n", kv.me, kv.leaderInfo)
	if kv.leaderInfo.leaderId != kv.me {
		reply.Err = ErrWrongLeader
		reply.LeaderIdx = kv.leaderInfo.leaderId
		return
	}
	// I'm the leader if i don't crash, so I can handle the request without lock
	// and send the reply.
	if value, exists := kv.kvStore[args.Key]; exists {
		reply.Value = value
		reply.Err = OK
	} else {
		reply.Err = ErrNoKey
	}
}

// PutAppend is the RPC handler for the PutAppend method.
func (kv *KVServer) PutAppend(args *PutAppendArgs, reply *PutAppendReply) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	if !kv.getLeader() {
		reply.Err = ErrNoLeader
		return
	}
	DPrintf(dServer, "%d, PutAppend: leaderInfo=%v\n", kv.me, kv.leaderInfo)
	if kv.leaderInfo.leaderId != kv.me {
		reply.Err = ErrWrongLeader
		reply.LeaderIdx = kv.leaderInfo.leaderId
		return
	}
	// send the message to raft
	done := kv.sendMsg(Op{OpName: args.Op, Key: args.Key, Value: args.Value, ID: kv.genOpID()})
	if !done {
		reply.Err = ErrTimeout
		return
	}
	DPrintf(dServer, "%d, PutAppend: %s %s %s succeeded", kv.me, args.Op,
		args.Key, args.Value)
	reply.Err = OK
}

// Faster way to get leader destination using kv.me
func (kv *KVServer) GetMe(args *GetMeArgs, reply *GetMeReply) {
	reply.Me = kv.me
}

// *********************************** Background Raft apply handler
// This function is called by Raft when a new command is applied.
func (kv *KVServer) raftApplyHandler() {
	for !kv.killed() {
		select {
		case applyMsg := <-kv.applyCh:
			if applyMsg.CommandValid {
				op := applyMsg.Command.(Op)
				DPrintf(dServer, "%d, RaftApplyHandler: applyMsg=%v\n", kv.me, applyMsg)
				// kv.mu.Lock()
				if op.OpName != NoOp {
					if op.OpName == PutOp {
						kv.kvStore[op.Key] = op.Value
					} else if op.OpName == AppendOp {
						if _, exists := kv.kvStore[op.Key]; exists {
							kv.kvStore[op.Key] += op.Value
						} else {
							kv.kvStore[op.Key] = op.Value
						}
					}
				}
				DPrintf(dServer, "%d, RaftApplyHandler: applied %s %s %s\n",
					kv.me, op.OpName, op.Key, op.Value)
				if ch, exists := kv.canReturn[op.ID]; exists {
					DPrintf(dServer, "%d, RaftApplyHandler: signaling done for op ID %d\n", kv.me, op.ID)
					ch <- struct{}{} // signal that the operation is done
				}
				// kv.mu.Unlock()
			} else if applyMsg.SnapshotValid {
				DPrintf(dServer, "%d, RaftApplyHandler: snapshot applied\n", kv.me)
				// handle snapshot (not implemented yet)
			}
		case <-time.After(3 * time.Second):
			continue // check every 3 seconds if killed
		}
	}
}

// *********************************** Miscellaneous

// the tester calls Kill() when a KVServer instance won't
// be needed again. for your convenience, we supply
// code to set rf.dead (without needing a lock),
// and a killed() method to test rf.dead in
// long-running loops. you can also add your own
// code to Kill(). you're not required to do anything
// about this, but it may be convenient (for example)
// to suppress debug output from a Kill()ed instance.
func (kv *KVServer) Kill() {
	atomic.StoreInt32(&kv.dead, 1)
	kv.rf.Kill()
	// Your code here, if desired.
}

func (kv *KVServer) killed() bool {
	z := atomic.LoadInt32(&kv.dead)
	return z == 1
}

// servers[] contains the ports of the set of
// servers that will cooperate via Raft to
// form the fault-tolerant key/value service.
// me is the index of the current server in servers[].
// the k/v server should store snapshots through the underlying Raft
// implementation, which should call persister.SaveStateAndSnapshot() to
// atomically save the Raft state along with the snapshot.
// the k/v server should snapshot when Raft's saved state exceeds maxraftstate bytes,
// in order to allow Raft to garbage-collect its log. if maxraftstate is -1,
// you don't need to snapshot.
// StartKVServer() must return quickly, so it should start goroutines
// for any long-running work.
func StartKVServer(servers []*labrpc.ClientEnd, me int, persister *raft.Persister, maxraftstate int) *KVServer {
	// call labgob.Register on structures you want
	// Go's RPC library to marshall/unmarshall.
	labgob.Register(Op{})
	labgob.Register(PutAppendArgs{})
	labgob.Register(PutAppendReply{})
	labgob.Register(GetArgs{})
	labgob.Register(GetReply{})
	labgob.Register(GetMeArgs{})
	labgob.Register(GetMeReply{})

	kv := new(KVServer)
	kv.me = me
	kv.maxraftstate = maxraftstate

	// You may need initialization code here.

	kv.applyCh = make(chan raft.ApplyMsg)
	kv.rf = raft.Make(servers, me, persister, kv.applyCh)

	// You may need initialization code here.

	kv.kvStore = make(map[string]string)
	kv.leaderInfo.leaderId = -1
	kv.leaderInfo.term = -1
	kv.dead = 0

	kv.canReturn = make(map[int]chan struct{})
	kv.opGen = 0

	go kv.raftApplyHandler()

	return kv
}
