package kvraft

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"6.824/labgob"
	"6.824/labrpc"
	"6.824/raft"
)

type OpType string

const (
	NoOp     OpType = "NoOp"
	PutOp    OpType = "Put"
	AppendOp OpType = "Append"
)

const (
	// Timeout for waiting for operation completion
	OperationTimeout = 500 * time.Millisecond
	// Ticker interval for periodic tasks
	PeriodicTaskInterval = 3000 * time.Millisecond
)

type Op struct {
	// Your definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
	OpName OpType
	Key    string
	Value  string
	ReqID  string // unique ID for the operation, used to deduplicate
}

type LeaderInfo struct {
	LeaderID int
	Term     int
}

type KVServer struct {
	mu      sync.Mutex
	me      int
	rf      *raft.Raft
	applyCh chan raft.ApplyMsg
	dead    int32 // set by Kill()

	maxraftstate int // snapshot if log grows this big

	// Your definitions here.
	leaderInfo LeaderInfo

	kvStore   *SafeChanMap // map[string]string // key-value store
	canReturn *SafeChanMap // map[string]chan struct{} // map of ReqIDs to channels to signal when the operation is done
	isDone    *SafeChanMap // map[string]bool // map of ReqIDs to whether the operation is done

	ticker *time.Ticker // ticker for periodic tasks, if needed
}

// *********************************** Input Validation

// validateGetRequest validates a Get request
func validateGetRequest(args *GetArgs) error {
	if args == nil {
		return fmt.Errorf("GetArgs cannot be nil")
	}
	if args.Key == "" {
		return fmt.Errorf("key cannot be empty")
	}
	return nil
}

// validatePutAppendRequest validates a PutAppend request
func validatePutAppendRequest(args *PutAppendArgs) error {
	if args == nil {
		return fmt.Errorf("PutAppendArgs cannot be nil")
	}
	if args.Key == "" {
		return fmt.Errorf("key cannot be empty")
	}
	if args.ReqID == "" {
		return fmt.Errorf("request ID cannot be empty")
	}
	if args.Op != string(PutOp) && args.Op != string(AppendOp) {
		return fmt.Errorf("invalid operation: %s", args.Op)
	}
	return nil
}

// *********************************** Private server APIs for encapsulated calls

// sendMsg sends a message to the raft server and waits for a reply.
func (kv *KVServer) sendMsg(op Op) bool {
	// lock is held outside.
	if v, ok := kv.isDone.Get(op.ReqID); ok && v.(bool) {
		// DPrintf(dServer, "%d, sendMsg: op=%v already done\n", kv.me, op)
		return true // operation already done, no need to send again
	}
	// send the message to raft
	// DPrintf(dServer, "%d, sendMsg: op=%v - H1", kv.me, op)
	ch := make(chan struct{}, 1)   // buffered channel to prevent deadlock
	kv.canReturn.Set(op.ReqID, ch) // create a channel to signal when the operation is done

	// cleanup function
	defer func() {
		kv.canReturn.Delete(op.ReqID)
	}()

	// send the operation to raft
	_, _, rfIsLeader := kv.rf.Start(op)
	// DPrintf(dServer, "%d, sendMsg: op=%v - H2", kv.me, op)
	if !rfIsLeader {
		return false // not the leader, cannot send the message
	}
	// wait for chan close
	select {
	case <-ch: // wait for the operation to be done
		DPrintf(dServer, "%d, sendMsg: op=%v done\n", kv.me, op)
		return true
	case <-time.After(OperationTimeout): // timeout
		// DPrintf(dServer, "%d, sendMsg: op=%v timeout\n", kv.me, op)
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
	// 	kv.me, rfTerm, rfLeader, kv.leaderInfo.Term, kv.leaderInfo.LeaderID)
	if kv.leaderInfo.Term == rfTerm && kv.leaderInfo.LeaderID == rfLeader {
		return true
	}
	// update leaderInfo
	DPrintf(dServer, "%d, getLeader: update leaderInfo to rfTerm=%d rfLeader=%d\n",
		kv.me, rfTerm, rfLeader)
	kv.leaderInfo.Term = rfTerm
	kv.leaderInfo.LeaderID = rfLeader
	// send no-ops if i am the leader
	if kv.leaderInfo.LeaderID == kv.me {
		// send no-op to myself with a timeout
		return kv.sendMsg(
			Op{
				OpName: NoOp,
				ReqID:  GenReqId(),
			})
	}
	return true
}

// *********************************** Public server APIs for client calls

// Get is the RPC handler for the Get method.
func (kv *KVServer) Get(args *GetArgs, reply *GetReply) {
	if err := validateGetRequest(args); err != nil {
		DPrintf(dError, "%d, Get: validation error: %v\n", kv.me, err)
		reply.Err = ErrBadRequest
		return
	}

	kv.mu.Lock()
	defer kv.mu.Unlock()
	if !kv.getLeader() {
		reply.Err = ErrNoLeader
		return
	}
	if kv.leaderInfo.LeaderID != kv.me {
		reply.Err = ErrWrongLeader
		return
	}
	// I'm highly probably the leader. Hope sendMsg will mostly succeed.
	// For Get operations, we use a unique request ID to ensure linearizability
	reqID := GenReqId()
	done := kv.sendMsg(
		Op{
			OpName: NoOp, // NoOp for Get
			Key:    args.Key,
			ReqID:  reqID,
		})
	if !done {
		reply.Err = ErrTimeout
		// DPrintf(dServer, "%d, Get: sendMsg timeout for key %s\n", kv.me, args.Key)
		return
	}
	DPrintf(dServer, "%d, Get: sendMsg succeeded for key %s\n", kv.me, args.Key)
	if value, exists := kv.kvStore.Get(args.Key); exists {
		reply.Value = value.(string)
		reply.Err = OK
	} else {
		reply.Err = ErrNoKey
	}
}

// PutAppend is the RPC handler for the PutAppend method.
func (kv *KVServer) PutAppend(args *PutAppendArgs, reply *PutAppendReply) {
	if err := validatePutAppendRequest(args); err != nil {
		DPrintf(dError, "%d, PutAppend: validation error: %v\n", kv.me, err)
		reply.Err = ErrBadRequest
		return
	}

	kv.mu.Lock()
	defer kv.mu.Unlock()
	if !kv.getLeader() {
		reply.Err = ErrNoLeader
		return
	}
	// DPrintf(dServer, "%d, PutAppend: leaderInfo=%v\n", kv.me, kv.leaderInfo)
	if kv.leaderInfo.LeaderID != kv.me {
		reply.Err = ErrWrongLeader
		return
	}
	// send the message to raft
	done := kv.sendMsg(
		Op{
			OpName: OpType(args.Op),
			Key:    args.Key,
			Value:  args.Value,
			ReqID:  args.ReqID,
		})
	if !done {
		reply.Err = ErrTimeout
		return
	}
	DPrintf(dServer, "%d, PutAppend: %s %s %s succeeded", kv.me, args.Op,
		args.Key, args.Value)
	reply.Err = OK
}

// *********************************** Background Raft apply handler
// This function is called by Raft when a new command is applied.
func (kv *KVServer) raftApplyHandler() {
	for {
		select {
		case applyMsg := <-kv.applyCh:
			if kv.killed() {
				return
			}
			if applyMsg.CommandValid {
				op := applyMsg.Command.(Op)
				// DPrintf(dServer, "%d, RaftApplyHandler: applyMsg=%v\n", kv.me, applyMsg)
				if v, ok := kv.isDone.Get(op.ReqID); ok && v.(bool) {
					// DPrintf(dServer, "%d, RaftApplyHandler: op %s %s %s already done\n",
					// 	kv.me, op.OpName, op.Key, op.Value)
					continue // operation already done, no need to apply again
				}
				kv.isDone.Set(op.ReqID, true) // mark the operation as done
				// kvStore update
				if op.OpName != NoOp {
					if op.OpName == PutOp {
						kv.kvStore.Set(op.Key, op.Value)
					} else if op.OpName == AppendOp {
						v, exists := kv.kvStore.Get(op.Key)
						if !exists {
							v = ""
						}
						kv.kvStore.Set(op.Key, v.(string)+op.Value)
					}
				}
				DPrintf(dServer, "%d, RaftApplyHandler: applied %s %s %s\n",
					kv.me, op.OpName, op.Key, op.Value)
				// signal that the operation is done.
				if ch, exists := kv.canReturn.Get(op.ReqID); exists {
					// DPrintf(dServer, "%d, RaftApplyHandler: signaling done for op ID %s\n", kv.me, op.ReqID)
					select {
					case ch.(chan struct{}) <- struct{}{}:
						// successfully signaled
					default:
						// channel is full or closed, skip
					}
				}
			} else if applyMsg.SnapshotValid {
				DPrintf(dServer, "%d, RaftApplyHandler: snapshot applied\n", kv.me)
				// handle snapshot (not implemented yet)
			}
		case <-kv.ticker.C:
			if kv.killed() {
				return
			}
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
	DPrintf(dServer, "%d, Killing server", kv.me)
	atomic.StoreInt32(&kv.dead, 1)
	kv.rf.Kill()
	kv.ticker.Stop()
	// Your code here, if desired.
	DPrintf(dServer, "%d, Server Killed", kv.me)
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

	kv := new(KVServer)
	kv.me = me
	kv.maxraftstate = maxraftstate

	// You may need initialization code here.

	kv.applyCh = make(chan raft.ApplyMsg)
	kv.rf = raft.Make(servers, me, persister, kv.applyCh)

	// You may need initialization code here.

	kv.leaderInfo.LeaderID = -1
	kv.leaderInfo.Term = -1
	kv.dead = 0

	kv.kvStore = NewSafeChanMap()
	kv.canReturn = NewSafeChanMap()
	kv.isDone = NewSafeChanMap()

	kv.ticker = time.NewTicker(PeriodicTaskInterval) // ticker for periodic tasks, if needed

	go kv.raftApplyHandler()

	return kv
}
