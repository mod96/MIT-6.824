package kvraft

const (
	OK             = "OK"
	ErrNoKey       = "ErrNoKey"
	ErrWrongLeader = "ErrWrongLeader"
	ErrTimeout     = "ErrTimeout"
	ErrNoLeader    = "ErrNoLeader"
)

type Err string

type ReplyWithLeaderIdx interface {
	GetLeaderIdx() int
	GetErr() Err
	Clear()
}

// Put or Append
type PutAppendArgs struct {
	Key   string
	Value string
	Op    string // "Put" or "Append"
	// You'll have to add definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
}

type PutAppendReply struct {
	Err       Err
	LeaderIdx int
}

func (reply *PutAppendReply) GetLeaderIdx() int {
	return reply.LeaderIdx
}
func (reply *PutAppendReply) GetErr() Err {
	return reply.Err
}
func (reply *PutAppendReply) Clear() {
	*reply = PutAppendReply{}
}

type GetArgs struct {
	Key string
	// You'll have to add definitions here.
}

type GetReply struct {
	Err       Err
	Value     string
	LeaderIdx int
}

func (reply *GetReply) GetLeaderIdx() int {
	return reply.LeaderIdx
}
func (reply *GetReply) GetErr() Err {
	return reply.Err
}
func (reply *GetReply) Clear() {
	*reply = GetReply{}
}

type GetMeArgs struct {
}
type GetMeReply struct {
	Me int
}
