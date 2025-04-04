# 0. Reading: Chain Replication

Chain Replication is a protocol designed to provide strong consistency, high availability, and scalability for storage systems, which sit somewhere between file systems and database systems. At its core, the system organizes a group of replicas in a linear chain structure, with clearly defined roles for each replica based on its position in the chain. The first node in the chain is called the head, and the last is the tail. Clients interact with these endpoints: write operations are directed to the head, while read operations are served only by the tail.

<p align="center">
    <img src="img/l11-0.PNG" width="50%" />
</p>

The write path in Chain Replication is sequential and straightforward. When a client issues a write, it first goes to the head of the chain. The head applies the write locally and forwards it to the next node in the chain. This process continues down the chain, with each replica applying the write in the same order and passing it along, until it reaches the tail. Once the tail replica processes the update, it sends an acknowledgment back to the client. This design ensures that a write is considered committed—and thus visible to reads—only after it has been fully applied by all replicas in the chain. As a result, Chain Replication provides strong consistency by guaranteeing that the tail always reflects the most recent, fully replicated state.

Reads are deliberately restricted to the tail replica. This decision simplifies the consistency model: clients can safely read from the tail without the risk of observing stale or partially applied updates. Although this limits read scalability to a single node (the tail), it ensures that all clients see a linearizable view of the system state.

Failure handling in Chain Replication is orchestrated by an external master service. If a replica crashes, the system reconfigures the chain by excluding the failed node and possibly replacing it with a backup. To preserve consistency, the reconfiguration process includes repairing any missing updates on the new node before resuming normal operation. During this period, write operations may be paused to prevent inconsistencies or loss of data.

The original paper introduces state transition diagrams to model the behavior of each replica and to rigorously prove safety properties. Each replica maintains a log of pending updates, and transitions between states as it receives, processes, and forwards each update. Although this formalism adds clarity for proving correctness, it can make the protocol seem more complex than it is. The key idea is simple: enforce a strict order of operations and ensure that reads reflect only fully acknowledged writes.

Chain Replication stands out in comparison to systems like Primary-Backup. In Primary-Backup, the primary server might reply to a client before the backup confirms the write, which risks exposing unreplicated state. In contrast, Chain Replication requires that the tail, which has seen the write last, confirm it to the client—ensuring that any acknowledged write is fully durable and visible to subsequent reads.


# 1. Lecture 11

## 1.1. Overview
Chain Replication (CR) is a protocol that supports high throughput and availability in distributed systems. Originally introduced in the 2004 OSDI paper by van Renesse and Schneider, Chain Replication improves upon the traditional primary/backup (P/B) replication model by offering stronger consistency guarantees with more efficient resource utilization.

## 1.2. Replicated State Machines: Two Approaches**
There are generally two methods for achieving replicated state machines:
1. Run all operations through consensus algorithms like Raft or Paxos (as seen in Lab 3).
2. Use a fault-tolerant configuration server (e.g., Zookeeper or Raft-based) to manage configuration and employ simpler replication techniques like primary-backup or chain replication for data replication.

The second approach separates concerns. The configuration server handles complex consensus and split-brain prevention, while the data replication protocol is lightweight and often easier to implement and optimize. This structure enables efficient failover and performance tuning in large-scale systems. Paxos or Raft are typically used for the configuration service because they are better equipped to deal with split-brain scenarios. Meanwhile, CR or P/B are often used for the replication layer, as they are simpler and better suited for large-scale data movement.

## 1.3. Primary-Backup Replication Review
Primary-backup is a well-established method. The primary assigns operation sequence numbers, applies the update locally, and forwards it to backups in parallel. The primary waits for all backups to acknowledge the update before responding to the client. Reads are served by the primary without needing coordination with backups.

This model can tolerate up to N-1 failures (with N replicas), which is better than quorum-based systems in terms of fault tolerance. Failures are handled by the configuration server, which reassigns roles and manages configuration updates. The system is sometimes known as ROWA (Read One, Write All), since writes must go to all replicas, but reads only to one.

Key properties any strong replication scheme must satisfy include:
1. No exposure of uncommitted data.
2. Agreement across replicas after recovery.
3. Avoidance of split-brain scenarios.
4. Ability to recover from total failure.

These properties require careful coordination between the data plane and the configuration service. The configuration server must include at least one replica from the previous configuration to ensure safety and correctness after failover.

## 1.4. Limitations of Primary-Backup
Primary-backup has a few shortcomings:
- The primary bears all client-facing work and communication load.
- If the primary fails, re-synchronizing backups can be complex.
- Backup consistency and split-brain avoidance require strict policies enforced by the configuration service.

## 1.5. Chain Replication: The Basics
Chain Replication restructures the responsibilities among replicas. Replicas are arranged in a linear chain, where the head receives client updates, and the tail serves client reads. Update operations flow sequentially from head to tail. Each replica persists and forwards updates in order. Only the tail acknowledges the client, ensuring the update has been fully replicated.

This structure reduces the burden on the head (less network I/O) and distributes client-facing work between the head (writes) and tail (reads). It guarantees strong consistency and facilitates recovery by preserving the ordering of operations across replicas.

Chain Replication assumes **fail-stop** behavior and relies on the configuration server to detect failures and reconfigure the chain. Before reconfiguration, updates may halt if the chain is broken due to partition or failure. After reconfiguration, the system resumes operation using only the reachable servers.

## 1.6. Handling Failures in Chain Replication
- **Head failure:** The configuration server appoints the second replica as the new head. In-flight updates may be lost but will be retried by clients.
- **Tail failure:** The preceding replica becomes the new tail. The tail must only respond after receiving all prior updates. Clients re-send updates if acknowledgments are missing.
- **Middle replica failure:** The configuration service connects the previous and next replicas, and the previous node replays any missing updates.

Each replica retains updates even after forwarding, freeing them only when receiving an acknowledgment from the successor. This ensures that failures don’t cause data loss and that recovery is deterministic. In the case of lost acks, retries are safe because updates are applied in strict order. If the tail's ack is lost, the previous replica will retry until it receives the ack.

## 1.6. Correctness Argument
Chain Replication avoids exposing uncommitted updates since reads come from the tail, which only sees updates after all replicas have applied them. Clients receive write acknowledgments only when the tail processes the update, ensuring that no update can vanish after a fault.

## 1.7. Adding New Replicas
When extending the chain to restore replication after a failure, the new replica is added to the tail. The old tail transfers its state to the new tail. This can be slow if done naively. A better method is to transfer a snapshot in advance and freeze updates briefly to synchronize recent changes. This approach is similar to ZooKeeper's fuzzy snapshot strategy.

## 1.8. Partition Handling
Partition tolerance is coordinated entirely by the configuration server. It selects a new head and tail based on observed server liveness. Servers must respect configuration changes, and leases are used to prevent old tails from serving reads after reconfiguration. However, the original CR paper does not specify mechanisms to prevent old heads or tails from continuing operation. A common solution is for replicas to reject requests from old heads or use leasing mechanisms to prevent stale reads.

## 1.9. Chain Replication vs. Other Models
- Compared to P/B, Chain Replication splits work more effectively and simplifies failover synchronization.
- Compared to quorum systems (e.g., Raft, Paxos), CR tolerates more failures (N-1 vs. N/2), but requires a separate configuration service and is more sensitive to slow or faulty replicas. Quorum-based systems like Raft are more resilient to temporary faults or slowness in a minority of servers, and can continue to serve clients without interruption. CR must pause to reconfigure.

## 1.10. Sharded Systems
In large-scale systems, data is partitioned across multiple chains (shards). A naïve layout dedicates a unique server trio to each shard, leading to load imbalance and slow recovery. A better approach ("rndpar") uses many more shards than servers, assigning each server to multiple shards in varying roles (head, tail, middle). This balances load and enables parallel recovery.

For example, consider three chains mapped across three servers:
- C1: S1 → S2 → S3
- C2: S2 → S3 → S1
- C3: S3 → S1 → S2
  
This design evenly spreads client-facing load across all servers.

## 1.11. Takeaways from Sections 5.1 to 5.4
- Chain throughput matches P/B, as both are limited by the head/primary CPU.
- Chain better distributes client work and reduces network load at the head.
- Randomized shard-to-server mapping allows fast parallel repair and load balancing.
- There is a tradeoff: randomized placement increases the probability that multiple failures can wipe out all replicas of a shard.

## 1.12. Fail-Stop Assumption
The CR paper assumes fail-stop behavior, meaning servers crash and stop, without arbitrary behavior or corruption. In practice, hardware and software are not perfectly fail-stop, but systems are often designed to behave this way by using checksums, ECC memory, and careful software design. More robust systems like PBFT exist for Byzantine fault tolerance, but they are more complex.


---

# 2. There is CRAQ

## 2.1. Overview
CRAQ (Chain Replication with Apportioned Queries) is an extension of Chain Replication (CR) designed to overcome a key limitation in the original model: the restriction that all reads must be served from the tail of the replica chain. CRAQ allows reads from any replica in the chain while preserving strong consistency. This enhancement addresses the read scalability bottleneck present in standard CR systems and improves latency for read-dominant workloads.

## 2.2. Background: Chain Replication Recap
In Chain Replication, replicas are organized in a linear sequence. Clients send write requests to the head of the chain, and the update is forwarded through each replica until it reaches the tail. Only after the tail applies the update does it respond to the client, ensuring that all replicas have applied the operation in the same order. For strong consistency, read requests are always directed to the tail, which has the most up-to-date state.

While CR guarantees linearizability and fault tolerance, it suffers from performance limitations in read-heavy environments, as the tail becomes a single read bottleneck. CRAQ improves upon this by enabling all replicas to participate in serving reads without sacrificing consistency.

## 2.2. CRAQ Design and Mechanism
CRAQ retains the same write propagation semantics as Chain Replication: writes travel sequentially from head to tail, with each replica persisting the update before forwarding it. However, CRAQ introduces an important addition: **dirty flags** on a per-key basis.

When a replica receives a write for a given key, it marks that key as **dirty**. This means the update has been applied locally but has not yet been acknowledged by the tail. Only when the tail receives the update and sends back an acknowledgment does the dirty flag get cleared at each upstream replica.

Each CRAQ replica can serve a read for a given key if that key is not marked dirty. If a replica receives a read for a key that is currently dirty, it can either:
- Forward the request to the tail, which is guaranteed to have the latest committed state, or
- Block the read until the dirty flag is cleared (i.e., the update has been acknowledged).

This mechanism ensures that no replica ever serves a stale or uncommitted value, preserving **linearizability** while significantly enhancing read scalability.

## 2.3. Implementation Details
CRAQ replicas track dirty keys using metadata structures such as hash tables or bitmaps. The tail includes information about which keys have been acknowledged in its response, and upstream replicas use this to clear the dirty flags.

To handle concurrent writes and reads efficiently, CRAQ can incorporate **version numbers** or **timestamps** with updates. These versions help ensure that out-of-order acknowledgments or duplicated messages do not result in incorrect state.

In practice, the number of dirty keys at any one time is small relative to the total dataset, particularly in read-heavy workloads. This means most reads can be served immediately by the nearest replica.

## 2.4. Performance Benefits
CRAQ demonstrates substantial performance improvements in systems with skewed read distributions, such as those following a Zipfian pattern. By allowing reads from all replicas, CRAQ reduces the load on the tail and balances read traffic across the system.

The evaluation in the paper shows that CRAQ outperforms standard CR in terms of read latency and throughput under various workloads, especially when write contention is low.

## 2.5. Tradeoffs and Considerations
While CRAQ improves read scalability, it introduces additional complexity:
- Maintaining per-key dirty state adds memory and bookkeeping overhead.
- Read operations may block or redirect if they target a dirty key.
- The system must be carefully engineered to handle race conditions between reads, writes, and acknowledgment propagation.

Despite these challenges, CRAQ remains a practical and performant approach for distributed systems requiring both strong consistency and high read throughput.

## 2.6. Sample Code

Code from [https://github.com/despreston/go-craq.git](https://github.com/despreston/go-craq.git)

```go
// Write adds an object to the chain. If the node is not the tail, the Write is
// forwarded to the next node in the chain. If the node is tail, the object is
// marked committed and a Commit message is sent to the predecessor in the
// chain.
func (n *Node) Write(key string, val []byte, version uint64) error {
	n.log.Printf("Node RPC Write() %s version %d to store\n", key, version)

	if err := n.store.Write(key, val, version); err != nil {
		n.log.Printf("Failed to write. %v\n", err)
		return err
	}

	// If this isn't the tail node, the write needs to be forwarded along the
	// chain to the next node.
	if !n.IsTail {
		next := n.neighbors[transport.NeighborPosNext]
		if err := next.rpc.Write(key, val, version); err != nil {
			n.log.Printf("Failed to send to successor during Write. %v\n", err)
			return err
		}
		return nil
	}

	// At this point it's assumed this node is the tail.

	if err := n.commit(key, version); err != nil {
		n.log.Printf("Failed to mark as committed in Write. %v\n", err)
		return err
	}

	// Start telling predecessors to mark this version committed.
	n.sendCommitToPrev(key, version)
	return nil
}
// Read returns values from the store. If the store returns ErrDirtyItem, ask
// the tail for the latest committed version for this key. That ensures that
// every node in the chain returns the same version.
func (n *Node) Read(key string) (string, []byte, error) {
	item, err := n.store.Read(key)

	switch err {
	case store.ErrNotFound:
		return "", nil, errors.New("key doesn't exist")
	case store.ErrDirtyItem:
		_, v, err := n.neighbors[transport.NeighborPosTail].rpc.LatestVersion(key)
		if err != nil {
			n.log.Printf(
				"Failed to get latest version of %s from the tail. %v\n",
				key,
				err,
			)
			return "", nil, err
		}

		item, err = n.store.ReadVersion(key, v)
		if err != nil {
			return "", nil, err
		}
	}

	return key, item.Value, nil
}
```

```go
type Storer interface {
	// Read an item from the store by key. If there is an uncommitted (dirty)
	// version of the item in the store, it returns a ErrDirtyItem error. If
	// no item exists for that key it returns a ErrNotFound error.
	Read(key string) (*Item, error)

	// Write a new item to the store.
	Write(key string, val []byte, version uint64) error

	// Commit a version for the given key. All items with matching key and older
	// than version are cleared.
	Commit(key string, version uint64) error

	// ReadVersion finds an item for the given key with the matching version. If
	// no item is found for that version of key, ErrNotFound is returned
	ReadVersion(key string, version uint64) (*Item, error)

	// AllNewerCommitted returns all committed items who's key is not in
	// versionsByKey or who's version is higher than the versions in
	// versionsByKey.
	AllNewerCommitted(versionsByKey map[string]uint64) ([]*Item, error)

	// AllNewerDirty returns all uncommitted items who's key is not in
	// versionsByKey or who's version is higher than the versions in
	// versionsByKey.
	AllNewerDirty(versionsByKey map[string]uint64) ([]*Item, error)

	// AllDirty returns all uncommitted items.
	AllDirty() ([]*Item, error)

	// AllCommitted returns all committed items.
	AllCommitted() ([]*Item, error)
}
```
