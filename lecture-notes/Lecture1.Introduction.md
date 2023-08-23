## Course Components

### Lectures:
  big ideas, paper discussion, lab guidance
  will be video-taped, available online

### Papers:
  there's a paper assigned for almost every lecture
  research papers, some classic, some new
  problems, ideas, implementation details, evaluation
  please read papers before class!
  each paper has a short question for you to answer
  and we ask you to send us a question you have about the paper
  submit answer and question before start of lecture

### Labs:
  goal: deeper understanding of some important ideas
  goal: experience with distributed programming
  first lab is due a week from Friday
  one per week after that for a while

- Lab 1: distributed big-data framework (MapReduce, as described in the paper)
- Lab 2: fault tolerance library using replication (replication using protocol `Raft` making replicated state machines on many machines)
- Lab 3: a simple fault-tolerant database 
- Lab 4: scalable database performance via sharding (to get performance through replication, we need sharding)


## Main Topics

This is a course about infrastructure for applications.

  - Storage.
  - Communication. 
    - frameworks orchestrating applications
  - Computation. 
    - RPC

To specify the Topics:

### Topic1: Fault Tolerance
  1000s of servers, big network -> always something broken We'd like to hide these failures from the application.
    
- High availability: service continues despite failures. 
  - Replication (If one server crashes, can proceed using the other(s). Labs 2 and 3)
- Recoverability: even if one machine die, we can recover from that.
  - Logging/Transactions to durable storage


### Topic2: Consistency
General-purpose infrastructure needs well-defined behavior.

- E.g. "Get(k) yields the value from the most recent Put(k,v)." 
  - This is consistent in sequential computation. But In concurrent computation, it could not. we'll learn eventual consistency, ...


### Topic3: Performance
The goal: scalable throughput (Labs 1, 4)

Nx servers -> Nx total throughput via parallel CPU, disk, net.

Scaling gets harder as N grows:
- Load imbalance.
- Slowest-of-N latency. (tail latency)
- throughput
- Some things don't speed up with N: initialization, interaction.
  

### Topic4: Tradeoffs (Topics 1~3)
Fault-tolerance, consistency, and performance are enemies.

Fault tolerance and consistency require communication
- e.g., send data to backup
- e.g., check if my data is up-to-date
- communication is often slow and non-scalable
  
Many designs provide only weak consistency, to gain speed.
- e.g. Get() does *not* yield the latest Put()!
- Painful for application programmers but may be a good trade-off.
  
We'll see many design points in the consistency/performance spectrum.

### Topic5: Implementation
  RPC, threads, concurrency control, configuration.
  The labs...


## Map Reduce

Abstract view of a MapReduce job -- word count
```
Map(k, v)
    split v into words
    for each word w
        emit(w, "1")

Reduce(k, v_set)
    emit(len(v_set))
```
```
  Input1 -> Map -> a,1 b,1
  Input2 -> Map ->     b,1
  Input3 -> Map -> a,1     c,1
                    |   |   |
                    |   |   -> Reduce -> c,1
                    |   -----> Reduce -> b,2
                    ---------> Reduce -> a,2
```
1. input is (already) split into M files
2. MR calls Map() for each input file, produces set of k2,v2
   "intermediate" data
   each Map() call is a "task"
3. when Maps are done,
   MR gathers all intermediate v2's for a given k2,
   and passes each key + values to a Reduce call
4. final output is set of <k2,v3> pairs from Reduce()s

\* Sorting, Concatenating things are done by the map reduce library.


### What about fault tolerance?
I.e. what if a worker crashes during a MR job?

We want to hide failures from the application programmer! Does MR have to re-run the whole job from the beginning? Why not?

*MR re-runs just the failed Map()s and Reduce()s.*

Can map run twice? Can reduce run twice? -> yes. if master noticed worker to be dead and it was just temporary network problem, one task would run twice. So MR relies on atomic file system.

Other failures/problems:
  * What if a single worker is very slow -- a "straggler"?
    perhaps due to flakey hardware.
    * coordinator starts a second copy of last few tasks.
  * What if a worker computes incorrect output, due to broken h/w or s/w?
    * too bad! MR assumes "fail-stop" CPUs and software.
  * What if the coordinator crashes?
    * not tolerable. need to re-run all.





