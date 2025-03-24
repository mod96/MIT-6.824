package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//
import (
	//	"bytes"

	"bytes"
	"sync"
	"sync/atomic"

	"6.824/labgob"
	"6.824/labrpc"

	// random sleep
	"math/rand"
	"time"
)

// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in part 2D you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh, but set CommandValid to false for these
// other uses.
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int
	// For 2D:
	SnapshotValid bool
	Snapshot      []byte
	SnapshotTerm  int
	SnapshotIndex int
}

type Log struct {
	Command interface{}
	Term    int
}

type State string

const (
	Candidate State = "Candidate"
	Follower  State = "Follower"
	Leader    State = "Leader"
)

// A Go object implementing a single Raft peer.
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()
	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.
	currentTerm int   // latest term server has seen
	votedFor    int   // candidateId that received vote in current term
	log         []Log // log entries

	commitIndex int // index of highest log entry known to be committed
	lastApplied int // index of highest log entry applied to state machine

	nextIndex  []int // for each server, index of the next log entry to send to that server
	matchIndex []int // for each server, index of highest log entry known to be replicated on server

	// extra states
	state         State
	leaderIdx     int
	lastHeartBeat time.Time // last heartbeat the leader has sent

	applyCh          chan ApplyMsg
	applyChCond      *sync.Cond
	applyChCondSkip  *bool
	sendLogsCond     *sync.Cond
	sendLogsCondSkip *bool

	X        int // log index starting point
	snapshot []byte
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {
	// Your code here (2A).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.currentTerm, rf.state == Leader
}

// **************************************************************************** Persistent
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
func (rf *Raft) getRaftState() []byte {
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(rf.currentTerm)
	e.Encode(rf.votedFor)
	e.Encode(rf.log)
	e.Encode(rf.X)
	data := w.Bytes()
	return data
}
func (rf *Raft) persist() {
	// Your code here (2C).
	rf.persister.SaveRaftState(rf.getRaftState())
}

func (rf *Raft) persistStateAndSnapshot() {
	rf.persister.SaveStateAndSnapshot(rf.getRaftState(), rf.snapshot)
}

// restore previously persisted state.
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (2C).
	r := bytes.NewBuffer(data)
	d := labgob.NewDecoder(r)
	var currentTerm int
	var votedFor int
	var log []Log
	var X int
	if d.Decode(&currentTerm) != nil ||
		d.Decode(&votedFor) != nil ||
		d.Decode(&log) != nil ||
		d.Decode(&X) != nil {
		DPrintf(dError, "S%d readPersist decoding error", rf.me)
	} else {
		rf.currentTerm = currentTerm
		rf.votedFor = votedFor
		rf.log = log
		rf.X = X
	}
	// Restart from snapshot
	rf.snapshot = rf.persister.ReadSnapshot()
	if rf.X > 0 {
		rf.lastApplied = rf.X
		rf.commitIndex = rf.X
		// applier is not ready yet. Just spawn goroutine and let it happen.
		go func() {
			rf.mu.Lock()
			defer rf.mu.Unlock()
			rf.applyCh <- ApplyMsg{
				SnapshotValid: true,
				Snapshot:      rf.snapshot,
				SnapshotTerm:  rf.log[0].Term,
				SnapshotIndex: rf.X,
			}
		}()
	}
}

// **************************************************************************** Snapshot
// A service wants to switch to snapshot.  Only do so if Raft hasn't
// have more recent info since it communicate the snapshot on applyCh.
func (rf *Raft) CondInstallSnapshot(lastIncludedTerm int, lastIncludedIndex int, snapshot []byte) bool {
	// Your code here (2D).
	return true
}

// the service says it has created a snapshot that has
// all info up to and including index. this means the
// service no longer needs the log through (and including)
// that index. Raft should now trim its log as much as possible.
func (rf *Raft) Snapshot(index int, snapshot []byte) {
	// Your code here (2D).
	// The index argument indicates the highest log entry that's reflected in the snapshot.
	// Caller(config.applierSnap) calls this synchronously. But this makes deadlock.
	go func() {
		rf.mu.Lock()
		defer rf.mu.Unlock()

		DPrintf(dSnap, "S%d, recieved snapshot with index %d", rf.me, index)
		if index <= rf.X {
			return
		}

		rf.log = rf.log[index-rf.X:]
		rf.X = index
		rf.snapshot = snapshot
		if rf.lastApplied < rf.X {
			rf.lastApplied = rf.X
		}
		rf.persistStateAndSnapshot()
		// To prevent index out of range, update rf.nextIndex
		// Now, InstallSnapshot ticker should send snapshot to other long-behind servers.
		// Not modifying rf.matchIndex here. InstallSnapshot ticker should handle that. It's important for updating commit index
		for serverIdx := range rf.peers {
			if rf.nextIndex[serverIdx] < index+1 {
				rf.nextIndex[serverIdx] = index + 1
			}
		}
	}()
}

type InstallSnapshotArgs struct {
	Term              int    // leader’s term
	LeaderId          int    // so follower can redirect clients
	LastIncludedIndex int    // the snapshot replaces all entries up through and including this index
	LastIncludedTerm  int    // term of lastIncludedIndex
	Data              []byte // raw bytes of the snapshot chunk, whole chunk
}

type InstallSnapshotReply struct {
	Term            int
	ClientLastIndex int
}

// Invoked by leader to send chunks of a snapshot to a follower.
// Leaders always send chunks in order.
func (rf *Raft) InstallSnapshot(args *InstallSnapshotArgs, reply *InstallSnapshotReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	needPersist := false
	defer func() {
		if needPersist {
			rf.persistStateAndSnapshot()
		}
	}()

	DPrintf(dSnap, "S%d, installSnapshot recieved from %d", rf.me, args.LeaderId)
	reply.Term = rf.currentTerm
	reply.ClientLastIndex = rf.X + len(rf.log) - 1
	if args.Term < rf.currentTerm {
		DPrintf(dLog, "S%d, installSnapshot recieved from %d - term %d is too old. rf.currentTerm: %d", rf.me, args.LeaderId, args.Term, rf.currentTerm)
		return
	}
	// Rules for Servers
	rf.lastHeartBeat = time.Now()
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.state = Follower
		rf.votedFor = args.LeaderId // if only leader can send this...
		needPersist = true
	}
	DPrintf(dSnap, "S%d, installSnapshot recieved from %d - original rf.X = %d, len(rf.log) = %d, args.LastIncludedIndex = %d", rf.me, args.LeaderId, rf.X, len(rf.log), args.LastIncludedIndex)
	// no offset & file saving implementation. skip description 2~5
	// description 6: If existing log entry has same index and term as snapshot’s last included entry, retain log entries following it and reply.
	// If snapshot is old, just return
	if args.LastIncludedIndex <= rf.X {
		return
	}
	// now we'll apply this snapshot
	needPersist = true
	// If snapshot is longer than mine, reset my log
	if rf.X+len(rf.log)-1 <= args.LastIncludedIndex {
		rf.log = []Log{{
			Command: nil,
			Term:    args.LastIncludedTerm,
		}}
	} else {
		// If snapshot is partial, cut my log
		rf.log = rf.log[args.LastIncludedIndex-rf.X:]
	}
	rf.X = args.LastIncludedIndex
	rf.snapshot = args.Data

	// description 8: Reset state machine using snapshot contents (and load snapshot’s cluster configuration)
	rf.applyCh <- ApplyMsg{
		SnapshotValid: true,
		Snapshot:      args.Data,
		SnapshotTerm:  args.LastIncludedTerm,
		SnapshotIndex: args.LastIncludedIndex,
	}

	rf.lastApplied = args.LastIncludedIndex
	rf.commitIndex = args.LastIncludedIndex
	DPrintf(dSnap, "S%d, installSnapshot recieved from %d - updated rf.X = %d, len(rf.log) = %d", rf.me, args.LeaderId, rf.X, len(rf.log))
}

func (rf *Raft) sendInstallSnapshot(server int) bool {
	DPrintf(dLeader, "S%d, Sending install snapshot to server %d with rf.X = %d", rf.me, server, rf.X)
	// Caller already gained lock
	args := &InstallSnapshotArgs{
		Term:              rf.currentTerm,
		LeaderId:          rf.me,
		LastIncludedIndex: rf.X,
		LastIncludedTerm:  rf.log[0].Term,
		Data:              rf.snapshot,
	}
	reply := &InstallSnapshotReply{}
	rf.mu.Unlock()
	ok := rf.peers[server].Call("Raft.InstallSnapshot", args, reply)
	rf.mu.Lock()
	if ok {
		// Rules for Servers
		if reply.Term > rf.currentTerm {
			rf.currentTerm = reply.Term
			rf.state = Follower
			rf.votedFor = -1
			rf.lastHeartBeat = time.Now()
			rf.persist()
		}
		// Am i still leader?
		if rf.state == Leader {
			if args.LastIncludedIndex <= reply.ClientLastIndex {
				// rf.matchIndex[server] = args.LastIncludedIndex
				rf.nextIndex[server] = min(reply.ClientLastIndex+1, rf.X+len(rf.log))
			} else {
				// rf.matchIndex[server] = args.LastIncludedIndex
				rf.nextIndex[server] = args.LastIncludedIndex + 1
			}
		}
	}
	return ok
}

// **************************************************************************** RequestVote
// example RequestVote RPC arguments structure.
// field names must start with capital letters!
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term         int
	CandidateId  int
	LastLogIndex int
	LastLogTerm  int
}

// example RequestVote RPC reply structure.
// field names must start with capital letters!
type RequestVoteReply struct {
	// Your data here (2A).
	Term        int
	VoteGranted bool
}

// example RequestVote RPC handler.
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	// 1. Reply false if term < currentTerm (§5.1)
	// 2. If votedFor is null or candidateId, and candidate’s log is at
	// least as up-to-date as receiver’s log, grant vote (§5.2, §5.4)
	rf.mu.Lock()
	defer rf.mu.Unlock()

	needPersist := false
	defer func() {
		if needPersist {
			rf.persist()
		}
	}()

	if args.Term < rf.currentTerm {
		DPrintf(dVote, "S%d, rejecting vote from %d, too old term %d", rf.me, args.CandidateId, args.Term)
		reply.Term = rf.currentTerm
		reply.VoteGranted = false
		return
	}
	// Rules for Servers
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.state = Follower
		rf.votedFor = -1
		needPersist = true
		// rf.lastHeartBeat = time.Now()
	}
	if (rf.votedFor == -1 || rf.votedFor == args.CandidateId) &&
		((rf.log[len(rf.log)-1].Term < args.LastLogTerm) ||
			(rf.log[len(rf.log)-1].Term == args.LastLogTerm && rf.X+len(rf.log)-1 <= args.LastLogIndex)) {
		DPrintf(dVote, "S%d, grant vote from %d, term %d. Where myLastTerm: %d, args.LastLogTerm: %d, rf.X: %d, len(rf.log): %d, args.LastLogIndex: %d", rf.me, args.CandidateId, args.Term, rf.log[len(rf.log)-1].Term, args.LastLogTerm, rf.X, len(rf.log), args.LastLogIndex)
		rf.currentTerm = args.Term
		rf.votedFor = args.CandidateId
		rf.lastHeartBeat = time.Now()

		reply.Term = rf.currentTerm
		reply.VoteGranted = true

		// Rules for Servers
		rf.state = Follower
		needPersist = true
		return
	}
	DPrintf(dVote, "S%d, rejecting vote from %d, term %d, no condition met", rf.me, args.CandidateId, args.Term)
	reply.Term = rf.currentTerm
	reply.VoteGranted = false
}

// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}

// **************************************************************************** AppendEntries
type AppendEntriesArgs struct {
	Term         int   // leader’s term
	LeaderId     int   // so follower can redirect clients
	PrevLogIndex int   // index of log entry immediately preceding new ones
	PrevLogTerm  int   // term of prevLogIndex entry
	Entries      []Log // log entries to store (empty for heartbeat; may send more than one for efficiency)
	LeaderCommit int   // leader’s commitIndex
}

type AppendEntriesReply struct {
	Term             int  // currentTerm, for leader to update itself
	Success          bool // true if follower contained entry matching prevLogIndex and prevLogTerm
	PrevLogIndex     int  // Could be modified by follower
	AppendEntriesLen int  // Could be modified by follower
}

func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	// Your code here (2A, 2B).
	// 1. Reply false if term < currentTerm (§5.1)
	// 2. Reply false if log doesn’t contain an entry at prevLogIndex
	// whose term matches prevLogTerm (§5.3)
	// 3. If an existing entry conflicts with a new one (same index
	// but different terms), delete the existing entry and all that
	// follow it (§5.3)
	// 4. Append any new entries not already in the log
	// 5. If leaderCommit > commitIndex, set commitIndex = min(leaderCommit, index of last new entry)
	rf.mu.Lock()
	defer rf.mu.Unlock()

	needPersist := false
	defer func() {
		if needPersist {
			rf.persist()
		}
	}()

	DPrintf(dLog, "S%d, appendEntries recieved from %d, rf.X: %d, len(rf.log): %d", rf.me, args.LeaderId, rf.X, len(rf.log))
	reply.Term = rf.currentTerm
	if args.Term < rf.currentTerm {
		DPrintf(dLog, "S%d, appendEntries recieved from %d - term %d is too old. rf.currentTerm: %d", rf.me, args.LeaderId, args.Term, rf.currentTerm)
		reply.Success = false
		return
	}
	// Rules for Servers
	rf.lastHeartBeat = time.Now()
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.state = Follower
		rf.votedFor = args.LeaderId // if only leader can send this...
		needPersist = true
	}
	// **1 Maybe this can help. rf.X is guaranteed to be committed (really? after delay?)
	reply.PrevLogIndex = args.PrevLogIndex
	if args.PrevLogIndex < rf.X {
		if args.PrevLogIndex+len(args.Entries) <= rf.X { // maybe delayed, even not containing committed entries can come
			DPrintf(dLog, "S%d, appendEntries recieved from %d - entries are too old. args.PrevLogIndex: %d, len(args.Entries): %d, rf.X: %d", rf.me, args.LeaderId, args.PrevLogIndex, len(args.Entries), rf.X)
			reply.Success = false
			return
		} else {
			args.Entries = args.Entries[rf.X-args.PrevLogIndex:]
		}
		args.PrevLogIndex = rf.X
		args.PrevLogTerm = rf.log[0].Term
		reply.PrevLogIndex = rf.X
		DPrintf(dLog, "S%d, appendEntries recieved from %d - modified. args.PrevLogIndex: %d, len(args.Entries): %d, rf.X: %d", rf.me, args.LeaderId, args.PrevLogIndex, len(args.Entries), rf.X)
	}
	reply.AppendEntriesLen = len(args.Entries)
	// description 2
	if args.PrevLogIndex >= rf.X+len(rf.log) || rf.log[args.PrevLogIndex-rf.X].Term != args.PrevLogTerm {
		DPrintf(dLog, "S%d, appendEntries recieved from %d - we need to go back from PrevLogIndex %d", rf.me, args.LeaderId, args.PrevLogIndex)
		reply.Success = false
		return
	}
	// description 3 & 4
	st := args.PrevLogIndex + 1
	for i, entry := range args.Entries {
		if st+i >= rf.X+len(rf.log) {
			rf.log = append(rf.log, args.Entries[i:]...)
			needPersist = true
			break
		}
		if rf.X <= st+i && st+i < rf.X+len(rf.log) &&
			(rf.log[st+i-rf.X].Term != entry.Term ||
				rf.log[st+i-rf.X].Command != entry.Command) {
			rf.log = append(rf.log[:st+i-rf.X], args.Entries[i:]...)
			needPersist = true
			break
		}
	}
	if args.Entries != nil {
		DPrintf(dTrace, "S%d, appendEntries recieved from %d - now log is len %d and commit is %d", rf.me, args.LeaderId, rf.X+len(rf.log), rf.commitIndex)
	}
	// description 5
	if args.PrevLogIndex >= args.LeaderCommit && args.LeaderCommit > rf.commitIndex {
		// send newly commited logs to applyCh
		for i := rf.lastApplied + 1; i < rf.X+len(rf.log) && i <= args.LeaderCommit; i++ {
			rf.applyCh <- ApplyMsg{
				CommandValid: true,
				Command:      rf.log[i-rf.X].Command,
				CommandIndex: i}
			rf.lastApplied++
		}
		DPrintf(dLog2, "S%d, appendEntries recieved from %d - update leadercommit from %d to %d", rf.me, args.LeaderId, rf.commitIndex, args.LeaderCommit)
		rf.commitIndex = min(args.LeaderCommit, rf.X+len(rf.log)-1)
	}
	reply.Success = true
}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}

// **************************************************************************** Service
// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	// Your code here (2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if rf.state != Leader {
		return -1, -1, false
	}

	index := rf.X + len(rf.log)
	term := rf.currentTerm
	isLeader := true

	DPrintf(dLeader, "S%d, recieved new log %v", rf.me, command)
	// add log for itself before sending AppendEntries. sendLogsToServers will do the job.
	rf.log = append(rf.log, Log{command, rf.currentTerm})
	rf.persist()

	// signal sendLogsToServers
	condBroadcastAndSetSkip(rf.sendLogsCond, rf.sendLogsCondSkip)
	DPrintf(dLeader, "S%d, nextIndex are %v while matchIndex are %v", rf.me, rf.nextIndex, rf.matchIndex)
	DPrintf(dTrace, "S%d, log is len %d and commit is %d", rf.me, rf.X+len(rf.log), rf.commitIndex)

	return index, term, isLeader
}

func (rf *Raft) sendLogsToServers() {
	rf.mu.Lock()
	me := rf.me
	rf.mu.Unlock()

	for serverIdx := range rf.peers {
		if serverIdx == me {
			continue
		}

		go func(serverIdx int) {
			defer DPrintf(dInfo, "S%d, kill sendLogsToServer %d", rf.me, serverIdx)
			// If last log index ≥ nextIndex for a follower: send
			// AppendEntries RPC with log entries starting at nextIndex
			// • If successful: update nextIndex and matchIndex for follower
			// • If AppendEntries fails because of log inconsistency:
			// decrement nextIndex and retry

			for !rf.killed() {
				retry := false
				rf.sendLogsCond.L.Lock()
				// only proceed when it's leader and there are more logs to send
				for rf.state != Leader || rf.X+len(rf.log) == 1 || rf.nextIndex[serverIdx] >= rf.X+len(rf.log) {
					DPrintf(dInfo, "S%d, i have nothing to do with sending logs to %d", rf.me, serverIdx)
					condWaitOrSkip(rf.sendLogsCond, rf.sendLogsCondSkip)
					if rf.killed() {
						rf.sendLogsCond.L.Unlock()
						return
					}
				}
				DPrintf(dLeader, "S%d, sending log(s) to %d with nextIndex %d", rf.me, serverIdx, rf.nextIndex[serverIdx])

				appendEntriesArgs := AppendEntriesArgs{
					Term:         rf.currentTerm,
					LeaderId:     rf.me,
					PrevLogIndex: rf.nextIndex[serverIdx] - 1,
					PrevLogTerm:  rf.log[rf.nextIndex[serverIdx]-1-rf.X].Term,
					Entries:      copyLog(rf.log[rf.nextIndex[serverIdx]-rf.X:]),
					LeaderCommit: rf.commitIndex,
				}
				// unlock
				rf.sendLogsCond.L.Unlock()

				reply := &AppendEntriesReply{}
				if !rf.sendAppendEntries(serverIdx, &appendEntriesArgs, reply) {
					retry = true
				} else {
					// Re-locking if we modify shared state
					rf.mu.Lock()
					// Rules for Servers
					if reply.Term > rf.currentTerm {
						rf.currentTerm = reply.Term
						rf.state = Follower
						rf.votedFor = -1
						rf.lastHeartBeat = time.Now()
						rf.persist()
					}
					// Am i still leader?
					if rf.state == Leader {
						// If succeeded, update
						if reply.Success {
							// long delayed response might try to change this later
							DPrintf(dLeader, "S%d, sending log(s) to %d succeed. PLI: %d, AEL: %d", rf.me, serverIdx, reply.PrevLogIndex, reply.AppendEntriesLen)
							if rf.nextIndex[serverIdx] < reply.PrevLogIndex+reply.AppendEntriesLen+1 {
								rf.matchIndex[serverIdx] = reply.PrevLogIndex + reply.AppendEntriesLen
								rf.nextIndex[serverIdx] = reply.PrevLogIndex + reply.AppendEntriesLen + 1
							}
							condBroadcastAndSetSkip(rf.applyChCond, rf.applyChCondSkip)
						} else if rf.nextIndex[serverIdx] > rf.X+1 {
							DPrintf(dLeader, "S%d, sending log(s) to %d failed", rf.me, serverIdx)
							// long delayed response might try to change this later
							if rf.nextIndex[serverIdx]-1 == reply.PrevLogIndex {
								rf.nextIndex[serverIdx] = reply.PrevLogIndex
							}
						} else {
							if !rf.sendInstallSnapshot(serverIdx) {
								retry = true
							}
						}
					}
					rf.mu.Unlock()
				}
				if retry {
					rf.mu.Lock()
					rf.sendLogsCondSkip = BoolPointer(true)
					rf.mu.Unlock()
				}
			}
		}(serverIdx)
	}
}

func (rf *Raft) commitConditionMetInteger() int {
	// If there exists an N such that N > commitIndex, a majority
	// of matchIndex[i] ≥ N, and log[N].term == currentTerm:
	// set commitIndex = N

	// lock is already held by the caller
	DPrintf(dLeader, "S%d Leader, checking new commit. currentCommit: %d, matchIndex: %v, nextIndex: %v", rf.me, rf.commitIndex, rf.matchIndex, rf.nextIndex)
	for i := rf.X + len(rf.log) - 1; i > rf.commitIndex; i-- {
		count := 1
		for serverIdx := range rf.peers {
			if serverIdx == rf.me {
				continue
			}
			if i <= rf.matchIndex[serverIdx] && rf.log[i-rf.X].Term == rf.currentTerm {
				count++
				if count > len(rf.peers)/2 {
					return i
				}
			}
		}
	}
	return -1
}

func (rf *Raft) updateCommitLoop() {
	defer DPrintf(dInfo, "S%d, kill updateCommitLoop", rf.me)

	for !rf.killed() {
		rf.applyChCond.L.Lock()
		newCommitIdx := -1
		// only proceed when it's leader and satisfies commit condition, proceed
		for {
			if rf.state == Leader {
				newCommitIdx = rf.commitConditionMetInteger()
				if newCommitIdx > rf.commitIndex {
					break
				}
				DPrintf(dLeader, "S%d Leader, commit not updating.", rf.me)
			}
			if !rf.killed() { // this could lead to stale thread, but let's leave it this way for simplicity
				condWaitOrSkip(rf.applyChCond, rf.applyChCondSkip)
			}
			if rf.killed() {
				var last5Logs []Log
				if len(rf.log) > 5 {
					last5Logs = rf.log[len(rf.log)-5:]
				} else {
					last5Logs = rf.log
				}
				DPrintf(dTrace, "S%d, kill state - term: %d, voted: %d, last5logs: %d, loglen: %d, commit: %d, lastApplied: %d", rf.me, rf.currentTerm, rf.votedFor, last5Logs, len(rf.log), rf.commitIndex, rf.lastApplied)

				rf.sendLogsCond.L.Unlock()
				return
			}
		}
		DPrintf(dLog2, "S%d, rf.commitIndex: %d, newCommitIdx: %d, rf.lastApplied: %d", rf.me, rf.commitIndex, newCommitIdx, rf.lastApplied)
		if rf.commitIndex < newCommitIdx {
			// applyCh for leader
			for i := rf.lastApplied + 1; i <= newCommitIdx; i++ {
				rf.applyCh <- ApplyMsg{
					CommandValid: true,
					Command:      rf.log[i-rf.X].Command,
					CommandIndex: i}
				rf.lastApplied++
			}
			DPrintf(dLog2, "S%d, update leadercommit from %d to %d", rf.me, rf.commitIndex, newCommitIdx)
			rf.commitIndex = newCommitIdx
		}

		rf.sendLogsCond.L.Unlock()
	}
}

// **************************************************************************** Kill
// the tester doesn't halt goroutines created by Raft after each test,
// but it does call the Kill() method. your code can use killed() to
// check whether Kill() has been called. the use of atomic avoids the
// need for a lock.
//
// the issue is that long-running goroutines use memory and may chew
// up CPU time, perhaps causing later tests to fail and generating
// confusing debug output. any goroutine with a long-running loop
// should call killed() to check whether it should stop.
func (rf *Raft) Kill() {
	DPrintf(dClient, "kill %d", rf.me)
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired.
	rf.mu.Lock()
	condBroadcastAndSetSkip(rf.sendLogsCond, rf.sendLogsCondSkip)
	condBroadcastAndSetSkip(rf.applyChCond, rf.applyChCondSkip)
	rf.mu.Unlock()
	// rf.waitGoroutines.Wait() // this slows things down and not realistic also
}
func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

// **************************************************************************** ticker
// The ticker go routine starts a new election if this peer hasn't received
// heartsbeats recently.
func (rf *Raft) ticker() {
	for !rf.killed() {
		// Your code here to check if a leader election should
		// be started and to randomize sleeping time using
		// time.Sleep().
		rf.mu.Lock()
		if rf.state == Leader {
			rf.mu.Unlock()
			rf.sendHeartbeat()
			time.Sleep(120 * time.Millisecond) // 120 milliseconds
		} else {
			rf.mu.Unlock()
			n := rand.Intn(150) + 200 // 200 ~ 350 milliseconds
			rf.sleepWhileCheckingLeader(n)

			rf.mu.Lock()
			if rf.killed() {
				rf.mu.Unlock()
				return
			}
			inElection := (rf.state == Follower || rf.state == Candidate) &&
				rf.lastHeartBeat.Before(time.Now().Add(-time.Duration(n)*time.Millisecond))

			if inElection {
				rf.currentTerm++
				rf.votedFor = rf.me
				rf.persist()
				rf.lastHeartBeat = time.Now()
				DPrintf(dVote, "S%d Candidate, requesting votes for term %d", rf.me, rf.currentTerm)

				rf.state = Candidate

				currentTerm := rf.currentTerm
				logIndex := rf.X + len(rf.log) - 1
				logTerm := rf.log[len(rf.log)-1].Term

				rf.mu.Unlock() // Unlock before spawning goroutines to avoid deadlocks

				count := 1

				for serverIdx := range rf.peers {
					if serverIdx == rf.me {
						continue
					}
					go func(serverIdx int, count *int) {
						reply := &RequestVoteReply{}
						rf.sendRequestVote(serverIdx, &RequestVoteArgs{
							Term:         currentTerm,
							CandidateId:  rf.me,
							LastLogIndex: logIndex,
							LastLogTerm:  logTerm,
						}, reply)

						// Re-locking if we modify shared state
						rf.mu.Lock()
						// Rules for Servers
						if reply.Term > rf.currentTerm {
							// *count = -len(rf.peers) // prevent split-brain since previous term accept arrives late -- NO!
							// When a candidate in Term 101 requests votes, a network delay may cause responses to arrive after the candidate has already moved to Term 102.
							// A vote reply for Term 101 arrives late and is processed after the node is already in Term 102.
							// Even though *count = -len(rf.peers) is set when reply.Term > rf.currentTerm, **this only happens if the response has a higher term.**
							// If the vote is granted for Term 101 (not a higher term), it still increments count.
							// Instead, rf.currentTerm != currentTerm check will do the job.
							rf.currentTerm = reply.Term
							rf.state = Follower
							rf.votedFor = -1
							rf.lastHeartBeat = time.Now()
							rf.persist()
							rf.mu.Unlock()
							return
						}

						// Ignore votes if the term has changed (stale vote)
						if rf.currentTerm != currentTerm {
							rf.mu.Unlock()
							return
						}

						if reply.VoteGranted {
							*count++
							if rf.state == Candidate && *count > len(rf.peers)/2 {
								// only place where it becomse leader
								DPrintf(dVote, "S%d Candidate -> Leader, i'm now a leader in term %d", rf.me, rf.currentTerm)
								rf.state = Leader
								rf.reInitializeVolatileStates()
								rf.mu.Unlock()
								rf.sendHeartbeat() // send heartbeat immediately to prevent stale leader elections
								return
							}
						}
						rf.mu.Unlock()
					}(serverIdx, &count)
				}
			} else {
				rf.mu.Unlock() // Unlock if no election was started
			}
		}
	}
}

func (rf *Raft) sendHeartbeat() {
	rf.mu.Lock()
	if rf.state != Leader {
		rf.mu.Unlock()
		return
	}
	// Notify sendLogsToServers periodically, in case it failed since network drop
	for serverIdx := range rf.peers {
		if serverIdx == rf.me {
			continue
		}
		if rf.X+len(rf.log) > 1 && rf.nextIndex[serverIdx] < rf.X+len(rf.log) {
			condBroadcastAndSetSkip(rf.sendLogsCond, rf.sendLogsCondSkip)
			break
		}
	}

	DPrintf(dLeader, "S%d Leader, checking heartbeats in term %d", rf.me, rf.currentTerm)
	DPrintf(dTrace, "S%d Leader, rf.nextIndex: %v, rf.X: %d, len(rf.log): %d", rf.me, rf.nextIndex, rf.X, len(rf.log))
	// Heartbeat
	rf.lastHeartBeat = time.Now()

	appendEntriesArgsSlice := []AppendEntriesArgs{}
	for serverIdx := range rf.peers {
		appendEntriesArgsSlice = append(appendEntriesArgsSlice, AppendEntriesArgs{
			Term:         rf.currentTerm,
			LeaderId:     rf.me,
			PrevLogIndex: rf.nextIndex[serverIdx] - 1,
			PrevLogTerm:  rf.log[rf.nextIndex[serverIdx]-1-rf.X].Term,
			Entries:      []Log{},
			LeaderCommit: rf.commitIndex})
	}

	rf.mu.Unlock()

	for serverIdx := range rf.peers {
		if serverIdx == rf.me {
			continue
		}
		go func(serverIdx int) {
			reply := &AppendEntriesReply{}
			rf.sendAppendEntries(serverIdx, &appendEntriesArgsSlice[serverIdx], reply)

			// Re-locking if we modify shared state
			rf.mu.Lock()
			// Rules for Servers
			if reply.Term > rf.currentTerm {
				rf.currentTerm = reply.Term
				rf.state = Follower
				rf.votedFor = -1
				rf.lastHeartBeat = time.Now()
				rf.persist()
			}
			rf.mu.Unlock()
		}(serverIdx)
	}
}

// **************************************************************************** Extra things
func (rf *Raft) reInitializeVolatileStates() {
	// Suppose lock is held by the caller
	rf.nextIndex = make([]int, len(rf.peers))
	for i := range rf.peers {
		// Makes faster to catch up. Especially for 2C.
		// For 2D, this made **1 necessary
		rf.nextIndex[i] = rf.X + 1
	}
	rf.matchIndex = make([]int, len(rf.peers))
	rf.sendLogsCond.Broadcast()
}

func (rf *Raft) sleepWhileCheckingLeader(n int) {
	sleepDuration := time.Duration(n) * time.Millisecond
	startTime := time.Now()

	for {
		remaining := sleepDuration - time.Since(startTime)
		if remaining <= 0 {
			break
		}

		sleepTime := 100 * time.Millisecond
		if remaining < sleepTime {
			sleepTime = remaining // Adjust sleep to not overshoot
		}

		time.Sleep(sleepTime)

		rf.mu.Lock()
		if rf.state == Leader {
			rf.mu.Unlock()
			break // Exit if we become leader
		}
		rf.mu.Unlock()
	}
}

// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	DPrintf(dClient, "make %d", me)
	rf := &Raft{}
	rf.peers = peers
	rf.me = me
	rf.persister = persister
	rf.applyCh = applyCh
	rf.applyChCond = sync.NewCond(&rf.mu)
	rf.applyChCondSkip = BoolPointer(false)
	rf.sendLogsCond = sync.NewCond(&rf.mu)
	rf.sendLogsCondSkip = BoolPointer(false)
	// Your initialization code here (2A, 2B, 2C).
	rf.currentTerm = 0
	rf.votedFor = -1
	rf.log = make([]Log, 1) // initialize log with an empty entry at index 0
	rf.X = 0

	rf.commitIndex = 0
	rf.lastApplied = 0

	rf.reInitializeVolatileStates()

	rf.state = Candidate
	rf.leaderIdx = -1

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())
	// start ticker goroutine to start elections
	go rf.ticker()
	rf.sendLogsToServers()
	go rf.updateCommitLoop()
	return rf
}
