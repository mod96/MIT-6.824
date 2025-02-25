# 0. Reading: ZooKeeper

## 0.1. Overview

ZooKeeper is a high-performance coordination service designed for distributed applications. It provides a simple yet powerful API that enables developers to implement various coordination primitives without relying on blocking operations like locks. By structuring data as wait-free hierarchical objects, ZooKeeper ensures fast and fault-tolerant coordination.

A key design goal of ZooKeeper is to offer **FIFO client request execution** and **linearizable writes**, allowing efficient processing of operations. The system's architecture leverages a pipelined model and a leader-based atomic broadcast protocol called Zab for consistency. ZooKeeper's read-heavy workloads benefit from local read processing and client-side caching, which is further enhanced by an event-driven watch mechanism.

Unlike traditional coordination services that enforce strong synchronization constraints, ZooKeeper provides relaxed consistency guarantees while maintaining high availability. Its replication-based approach enables scalability, allowing applications with numerous processes to rely on ZooKeeper for tasks such as configuration management, leader election, and group membership.

The paper highlights three main contributions:

**Coordination Kernel** – A wait-free coordination service that supports diverse distributed system needs.

**Coordination Recipes** – Demonstrations of how ZooKeeper can implement higher-level coordination primitives.

**Experience with Coordination** – Real-world use cases and performance evaluations showcasing ZooKeeper's effectiveness in large-scale systems.

Overall, ZooKeeper serves as a fundamental building block for distributed applications, offering a balance of performance, reliability, and flexibility.

## 0.2. Service

ZooKeeper offers a hierarchical namespace of data nodes, called `znodes`, which clients can manipulate through an API similar to a file system. The hierarchical structure is beneficial for organizing metadata and structuring distributed applications. Clients refer to znodes using UNIX-style paths (e.g., `/A/B/C`). There are two types of znodes:
- **Regular znodes:** Created and deleted explicitly by clients.
- **Ephemeral znodes:** Automatically deleted when the client session that created them ends.

Additionally, znodes can be created with a **sequential flag**, ensuring they are named with a monotonically increasing counter appended to their names.

ZooKeeper enables clients to register **watches** on znodes to receive notifications when data changes, eliminating the need for polling. Watches are one-time triggers associated with a session and are removed after triggering.

### 0.2.1. Data Model
ZooKeeper follows a hierarchical key-value model similar to a file system but optimized for metadata rather than general data storage. The model allows applications to use znodes for leader election, configuration management, and group membership. Clients can also track changes using metadata like timestamps and version counters.

<p align="center">
    <img src="img/l10-0.PNG" width="40%" />
</p>

### 0.2.2. Sessions
Clients interact with ZooKeeper through sessions. A session remains active as long as the client sends heartbeats within a specified timeout. If the session times out, ephemeral znodes created by that client are removed. Sessions allow seamless movement of clients across ZooKeeper servers while maintaining state.

### 0.2.3. Client API
The ZooKeeper API provides methods for creating, modifying, and monitoring znodes. Some key operations include:
- **create(path, data, flags):** Creates a znode with the specified path, storing the provided data.
- **delete(path, version):** Deletes a znode if it matches the expected version.
- **exists(path, watch):** Checks if a znode exists and sets a watch if requested.
- **getData(path, watch):** Retrieves data and metadata associated with a znode.
- **setData(path, data, version):** Updates a znode if the expected version matches.
- **getChildren(path, watch):** Lists the children of a znode.
- **sync(path):** Ensures all previous updates are propagated before proceeding.

The API offers both synchronous and asynchronous versions of these operations. The asynchronous API allows clients to issue multiple concurrent requests and execute tasks in parallel. ZooKeeper does not use handles for accessing znodes; instead, each request includes the full path, simplifying the API and reducing server-side state management.

Each update method includes a version check, enabling **conditional updates**. If the provided version does not match the current znode version, the update fails, preventing conflicts in distributed environments.

ZooKeeper's design ensures high availability, efficiency, and flexibility, making it a fundamental building block for distributed applications.

### 0.2.4. Guarantees

#### Ordering Guarantees
ZooKeeper provides two fundamental ordering guarantees:
1. **Linearizable Writes:** All update requests are serializable and respect precedence, ensuring a consistent state.
2. **FIFO Client Order:** Requests from a single client are executed in the order they were sent.

#### Ensuring Consistency (in Leader Election example)
When a new leader takes control of a system, it must update configuration parameters. ZooKeeper ensures consistency in such cases by using a designated `ready znode` to indicate when the configuration has been finalized. The leader deletes the `ready znode`, updates configurations, and then recreates the `ready znode`. Because of ZooKeeper’s ordering guarantees, any process that sees the `ready znode` must also see all prior configuration changes. If the leader fails before the `ready znode` is recreated, processes recognize that the configuration is incomplete and do not use it.

#### Watches and Notifications (in Leader Election example)
ZooKeeper guarantees that notifications arrive before a client can observe the state change, ensuring no inconsistencies. If a client watches a znode before a leader starts modifying it, ZooKeeper ensures that the client receives the notification before reading the new state.

#### Handling Out-of-Sync Replicas
ZooKeeper ensures consistency even when clients communicate through additional channels outside of ZooKeeper. If a client (A) updates a shared configuration and notifies another client (B) through a separate communication channel, (B) may read stale data if its ZooKeeper replica is behind. To prevent this, (B) can issue a **sync** request before reading, forcing the server to apply all pending updates before responding. This ensures that the read operation sees the latest data without the overhead of a full write operation.

#### Liveness and Durability Guarantees
ZooKeeper ensures the following guarantees:
- **Availability:** If a majority of ZooKeeper servers are active and can communicate, the service remains available.
- **Durability:** Once ZooKeeper successfully acknowledges a change request, that change persists even if failures occur, as long as a quorum of servers eventually recovers.

These guarantees make ZooKeeper highly reliable for distributed coordination tasks.

## 0.3. Examples

### 0.3.1. Primitives

#### Configuration Management
ZooKeeper allows dynamic configuration updates in a distributed system. Configuration is stored in a znode ($z_c$), which clients read with a watch flag set. When $z_c$ is updated, clients are notified and re-read the configuration. This ensures processes always use the latest configuration without needing continuous polling.

#### Rendezvous
ZooKeeper can coordinate master-worker setups where workers need to discover the master’s address dynamically. A client creates a **rendezvous znode** ($z_r$) that the master updates with its details. Workers read $z_r$ with a watch set, waiting for the master to publish its information. If $z_r$ is ephemeral, its deletion signals that the master is no longer available, prompting cleanup.

#### Group Membership
Ephemeral znodes simplify group membership tracking. A designated **group znode** ($z_g$) stores child znodes, each representing an active process. A process joins by creating an ephemeral child under $z_g$. When a process terminates, its znode is removed automatically, keeping the group list up to date. Clients monitoring group membership use watches to detect changes.

#### Simple Locks
ZooKeeper can implement a simple locking mechanism using ephemeral znodes. A client attempts to create a lock znode ($z_l$). If successful, it holds the lock; otherwise, it watches $z_l$ and retries upon deletion. While simple, this can cause a **herd effect** where all waiting clients attempt to acquire the lock simultaneously.

#### Simple Locks Without Herd Effect
A more efficient lock avoids the herd effect by using **sequential znodes**. Clients create sequential znodes (`l/lock-`), forming an ordered queue. Each client watches only the preceding znode, ensuring only one client wakes up when a lock is released.

```
Lock
1 n = create(l + “/lock-”, EPHEMERAL|SEQUENTIAL)
2 C = getChildren(l, false)
3 if n is lowest znode in C, exit
4 p = znode in C ordered just before n
5 if exists(p, true) wait for watch event
6 goto 2
Unlock
1 delete(n)
```

code from [public-github](https://github.com/tomaszstaniewicz/zookeeper-distributed-lock/tree/master)
```java
public class DistributedLock {
    private final ZooKeeper zk;
    private final String lockBasePath;
    private final String lockName;
    private String lockPath;

    public DistributedLock(ZooKeeper zk, String lockBasePath, String lockName) {
        this.zk = zk;
        this.lockBasePath = lockBasePath;
        this.lockName = lockName;
    }

    public void lock() throws IOException {
        try {
// lockPath will be different than (lockBasePath + "/" + lockName) becuase of the sequence number ZooKeeper appends
            lockPath = zk.create(lockBasePath + "/" + lockName,
                    null,
                    ZooDefs.Ids.OPEN_ACL_UNSAFE,
                    CreateMode.EPHEMERAL_SEQUENTIAL
            );

            final Object lock = new Object();
            synchronized (lock) {
                while (true) {
                    List<String> nodes = zk.getChildren(lockBasePath, new Watcher() {
                        public void process(WatchedEvent event) {
                            synchronized (lock) {
                                lock.notifyAll();
                            }
                        }
                    });
                    Collections.sort(nodes); // ZooKeeper node names can be sorted lexographically
                    if (lockPath.endsWith(nodes.get(0))) {
                        return;
                    } else {
                        lock.wait();
                    }
                }
            }
        } catch (KeeperException e) {
            throw new IOException(e);
        } catch (InterruptedException e) {
            throw new IOException(e);
        }
    }

    public void unlock() throws IOException {
        try {
            zk.delete(lockPath, -1);
            lockPath = null;
        } catch (KeeperException e) {
            throw new IOException(e);
        } catch (InterruptedException e) {
            throw new IOException(e);
        }
    }
}
```

#### Read/Write Locks
Read/write locks extend simple locks by distinguishing between read and write locks. Read locks allow multiple clients, provided no write lock exists. Clients requesting read locks watch preceding write znodes to ensure safety.

```
Write Lock
1 n = create(l + “/write-”, EPHEMERAL|SEQUENTIAL)
2 C = getChildren(l, false)
3 if n is lowest znode in C, exit
4 p = znode in C ordered just before n
5 if exists(p, true) wait for event
6 goto 2

Read Lock
1 n = create(l + “/read-”, EPHEMERAL|SEQUENTIAL)
2 C = getChildren(l, false)
3 if no write znodes lower than n in C, exit
4 p = write znode in C ordered just before n
5 if exists(p, true) wait for event
6 goto 3
```

#### Double Barrier
A **double barrier** synchronizes a set of processes at the beginning and end of a computation. A designated **barrier znode** (`b`) tracks participants. Processes join by creating znodes under `b` and exit when all znodes are removed. Watches efficiently notify processes when conditions are met.

These examples showcase ZooKeeper’s flexibility in implementing advanced coordination mechanisms in distributed applications.

### 0.3.2. Real-world applications

#### Fetching Service
Yahoo!’s **Fetching Service (FS)** is a critical component of its web crawler, which fetches billions of web documents. FS uses ZooKeeper for **configuration management** and **leader election**. By storing configuration metadata in ZooKeeper, fetchers can read from healthy servers and recover from master failures seamlessly.

#### Katta
**Katta** is a distributed indexing system that utilizes ZooKeeper for **group membership**, **leader election**, and **configuration management**. Katta divides indexing work into shards, which are assigned to slave nodes by a master. If a master fails, another node takes over. Similarly, slave nodes dynamically reassign work as failures occur, ensuring continuous indexing operations.

#### Yahoo! Message Broker
**Yahoo! Message Broker (YMB)** is a distributed publish-subscribe system managing thousands of topics across multiple servers. YMB uses ZooKeeper for **configuration metadata**, **failure detection**, and **group membership**. It employs a **primary-backup scheme** for message replication, ensuring reliability. The ZooKeeper layout for YMB includes:
- **Nodes directory:** Stores ephemeral znodes representing active servers.
- **Topics directory:** Stores znodes for each topic with child znodes indicating the primary and backup servers.
- **Control structures:** Monitors shutdown and migration statuses to manage cluster-wide operations.

This architecture enables YMB to maintain reliable message distribution while efficiently handling failures and leader transitions.


