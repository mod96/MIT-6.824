# 0. Reading: Consistency and Linearizability

https://anishathalye.com/testing-distributed-systems-for-linearizability/

### **What is Linearizability Checking?**  
Linearizability checking is a method used to verify whether a distributed system behaves correctly under concurrent operations. It ensures that every operation appears to execute **atomically and instantaneously** at some point between its invocation and response. This means that even if operations are happening in parallel, there should exist a **valid sequential order** that explains the observed results.

### **Why Do We Need Linearizability Checking?**  
Distributed systems must handle concurrency and failures while maintaining correctness. However, due to **network delays, message reordering, machine failures, and race conditions**, testing for correctness is **non-trivial**. Simple tests that check expected outputs often fail to detect subtle bugs that arise due to these concurrency issues.

Linearizability checking helps us:
1. **Ensure Consistency:** It verifies that operations follow a correct order, making the system predictable.
2. **Detect Subtle Bugs:** Bugs that appear only under rare conditions (e.g., simultaneous failures or network delays) can be exposed.
3. **Validate Implementations:** Even if a distributed algorithm is **correct in theory**, implementations may have concurrency bugs.
4. **Test Under Fault Injection:** Linearizability checking allows us to validate a system's behavior when faults like network partitions occur.

### **How Does Linearizability Checking Work?**  
1. **Record the Execution History:**  
   - Every operation’s invocation and response are recorded, including timestamps.
   - Example:
     ```
     Client 1: Put("x", "A") -----> Done
     Client 2: Get("x") -----> "A"
     ```

2. **Construct a Sequential Order:**  
   - The checker tries to find an order of operations that **matches** the sequential specification of the system.

3. **Verify Consistency:**  
   - If an order exists where all operations appear to have executed in sequence while still respecting concurrency constraints, the history is **linearizable**.
   - If no such order exists, the history is **not linearizable**, indicating a bug.

### **Example of Linearizable vs. Non-Linearizable History**
#### ✅ **Linearizable History**  
```
Client 1: Put("x", "A") -----> Done
Client 2: Get("x") -----> "A"
```
- This is valid because the `Get("x")` sees the last write.

#### ❌ **Non-Linearizable History**  
```
Client 1: Put("x", "A") -----> Done
Client 2: Put("x", "B") -----> Done
Client 3: Get("x") -----> "A"
```
- This **violates** linearizability because Client 3 should have seen `"B"`, as it was written **after** `"A"`.

### **Challenges in Linearizability Checking**
- **NP-Complete Complexity:** Finding a valid linearization requires searching through all possible orderings, making it computationally expensive.
- **Large Histories:** If a system has thousands of events, verifying all possible sequential orders becomes infeasible.
- **Efficiency Optimizations:** Tools like **Porcupine** use heuristics and optimizations to make checking practical.

### **Tools for Linearizability Checking**
- **Jepsen (Knossos):** Commonly used in distributed systems testing.
- **Porcupine:** A highly efficient linearizability checker written in Go.
- **TLA+:** Can be used for formal verification but is more complex.

### **Conclusion**
Linearizability checking is a **powerful technique** for verifying that a distributed system behaves as expected under concurrency. It is especially useful for catching subtle bugs that may not be apparent in simpler tests. Despite its computational challenges, it remains a **critical tool** in distributed system testing.

# 1. Lecture 9

## 1.1. Consistency and Linearizability

### Understanding Consistency Models
Consistency models define the relationship between different clients' views of a distributed system. In distributed key-value storage, consistency governs how operations like `put(k, v)` and `get(k)` behave under concurrent access. The key question is: given multiple `put` and `get` operations happening concurrently, what are the valid outcomes?

#### The Challenge of Consistency
Distributed storage systems face multiple challenges that impact consistency, including:
- **Concurrency:** Multiple clients performing operations simultaneously.
- **Replication:** Data is often stored across multiple nodes.
- **Caches:** Data may be temporarily stored at various locations.
- **Failures and Recovery:** Network partitions and machine crashes can affect data availability.
- **Retransmissions:** Requests may be retried after failures, leading to duplicate operations.

Without a well-defined consistency model, it becomes difficult for application developers to reason about the behavior of a system. For example, if a producer writes:
```python
put("result", 27)
put("done", True)
```
and a consumer reads:
```python
while get("done") != True:
    pause()
v = get("result")
```
can the consumer **always** be sure that `v == 27`? The answer depends on the consistency model the system provides.

### Linearizability: A Strong Consistency Model
#### What is Linearizability?
Linearizability is a **strong consistency model** that ensures operations appear to occur **instantaneously** at some point between their invocation and response. This means all operations are ordered in a way that respects real-time constraints and produces results consistent with a single sequential execution.

Linearizability guarantees that:
1. Operations take effect **in real-time order**.
2. The system behaves as if operations were executed **atomically** in some serial order.

This model aligns with programmer intuition but restricts many optimizations that could improve performance.

#### Understanding Histories and Linearization
A **history** is a sequence of operations, including invocation and response times. A history is **linearizable** if we can assign a **linearization point** (an instant in time between invocation and response) to each operation such that:
- The history's results match a **valid sequential execution**.
- Each operation's linearization point occurs **within its execution window** (between invocation and response).

##### Example 1: A Linearizable History
```
|-Wx1-| |-Wx2-|
  |---Rx2---|
    |-Rx1-|
```
This is linearizable because we can order the operations as:
```
Wx1 -> Rx1 -> Wx2 -> Rx2
```
where:
- `Wx1` (write x=1) happens first.
- `Rx1` (read x) sees 1.
- `Wx2` (write x=2) happens next.
- `Rx2` (read x) sees 2.

##### Example 2: A Non-Linearizable History
```
|-Wx1-| |----Wx2----|
  |--Rx2--|
            |-Rx1-|
```
Attempting to order the operations fails because:
- `Rx2` reads 2, meaning `Wx2` must have completed before it.
- `Rx1` reads 1, meaning `Wx1` must have completed after `Wx2`.
- This creates an impossible cycle.

Thus, no valid sequential execution exists, making the history **non-linearizable**.

### Why Linearizability Matters
Linearizability is essential for designing predictable distributed systems. It ensures:
- **Clients see fresh data** rather than stale values.
- **All clients observe operations in the same order**, preventing inconsistencies.
- **Atomic operations** like `compare-and-swap` function correctly.

However, enforcing linearizability comes with trade-offs:
- **Performance Overhead:** Requires coordination across replicas.
- **Availability Constraints:** Some operations may have to block until a global order is determined.

### Implementing Linearizability
#### Single-Server Model
A simple way to enforce linearizability is to have a **single server** processing requests sequentially. This guarantees a strict operation order but creates a **single point of failure** and performance bottleneck.

#### Primary-Backup Replication
A more practical approach is **primary-backup replication**:
1. The **primary** receives client requests and assigns a serial order.
2. The primary forwards operations to **backup replicas**.
3. The operation is considered complete only when **all backups confirm execution**.

This ensures linearizability, but clients **cannot read from backups** because they may be stale.

#### Raft and Consensus Protocols
Consensus protocols like **Raft** ensure all nodes agree on a **global order** for operations. Raft uses a **leader-based** approach where:
- The leader assigns a **log order** to client requests.
- Followers replicate operations in the same order.
- Only **committed operations** are visible to clients.

Raft guarantees linearizability at the cost of **increased communication overhead**.

### Alternative Consistency Models
While linearizability is ideal, many distributed systems adopt **weaker consistency models** for better performance and availability.

#### Eventual Consistency
In **eventual consistency**, updates propagate asynchronously, and replicas may temporarily diverge. Eventually, all replicas converge to a consistent state.
- Reads may return **stale values**.
- Different clients may see **updates in different orders**.
- Used in **Amazon DynamoDB, Cassandra**, and other highly available systems.

#### Sequential Consistency
Sequential consistency allows operations to be **ordered arbitrarily**, as long as all clients observe the same order. It relaxes the real-time requirement of linearizability.

#### Causal Consistency
Causal consistency ensures operations that are **causally related** appear in the same order for all clients, while independent operations may be reordered.

## Conclusion
Linearizability is a **gold standard** for correctness in distributed systems but comes with performance trade-offs. Systems like Raft and primary-backup replication ensure linearizability but at the cost of latency and coordination. Weaker models like eventual consistency prioritize availability but allow temporary inconsistencies.

Understanding these trade-offs helps in designing distributed systems that balance **correctness, performance, and availability** based on application needs.

