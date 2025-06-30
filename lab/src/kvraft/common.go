package kvraft

const (
	OK             = "OK"
	ErrNoKey       = "ErrNoKey"
	ErrWrongLeader = "ErrWrongLeader"
	ErrTimeout     = "ErrTimeout"
	ErrNoLeader    = "ErrNoLeader"
	ErrBadRequest  = "ErrBadRequest"
)

type Err string

type Reply interface {
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
	ReqID string // unique request ID to deduplicate requests
}

type PutAppendReply struct {
	Err Err
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
	Err   Err
	Value string
}

func (reply *GetReply) GetErr() Err {
	return reply.Err
}
func (reply *GetReply) Clear() {
	*reply = GetReply{}
}
