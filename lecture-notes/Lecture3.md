# Lecture3. GFS

## Storage Systems

### Why is it hard?

For high performance -> shard data across multiple servers 

-> how about server faults? -> replication for fault tolerance 

-> potential inconsistencies -> strong consistency protocol 

-> lower performance

\* Ideal consistency: Behave as if single system 

## GFS

Is a good case study that it uses replication, fault tolerance, consistency for performance. And it was successful, that it used 1000s servers. But it's not standard in these days. It uses one master, and can have inconsistencies.

Actually, gfs is a filesystem for map-reduce. When the time the paper released, disk technology was about 10~30 MB/s. And it is impressive for nowaday's disks too. 

(image from map reduce paper)
![](./img/l3_1.PNG)


GFS has these features:

- Big: large data set
- Fast: automatic sharding
- Global: all apps see same file system
- Fault tolerant: automatic fault tolerance


## Design

### What are the steps when client C wants to read a file?

![](./img/l3_gfs_archi.PNG)

  1. C sends filename and offset to coordinator (CO, master) (if not cached)
   
     CO has a `filename -> array-of-chunkhandle` table 

     and a `chunkhandle -> list-of-chunkservers` table (with handle version, primary/secondaries, leasetime)

  2. CO finds chunk handle for that offset
  3. CO replies with chunkhandle + list of chunkservers + version number
  4. C caches handle + chunkserver list + version number
  5. C sends request to nearest chunkserver
     with chunk handle + offset
  6. chunk server(S) checks version number if it's not stale and reads from chunk file on disk, returns to client

\* What master handles:
- filename -> array of chunk handles
- chunk handle -> version number(in stable storage), list of chunk servers, primary/secondaries, lease time
- log(in stable storage) + checkpoints(in stable storage) <- so that master can recover using these.

### What about writes? : write as some offset

![](./img/l3_gfs_archi_write.PNG)

0. For each chunk, the master designate one server as "primary". It increases version number of the chunk of the picked primary server.
1. C asks CO(master) about file's chunk at offset
2. CO tells C the primary + secondaries + version number
3. C sends data to all (just temporary...), waits for all replies (Each chunkserver uses LRU buffer cache until the data is used or aged out)
4. C asks P(primary) to write.
5. P checks that lease hasn't expired.
6. P writes its own chunk file (a Linux file).
7. P tells each secondary to write (command: copy temporary into chunk file)
8. P waits for all secondaries to reply, or timeout. Secondary can reply "error" e.g. out of disk space.
9. P tells C "ok" or "error".
10. (After few retries for 3~9) C retries from start if error. (At least once)

Here's how inconsistent content arise:
- Primary P updated its own state.
- But secondary S1 did not update (failed? slow? network problem?).
- Client C1 reads from P; Client C2 reads from S1. => they will see different results

### Consistency : write as some offset

![](./img/l3_consistency.PNG)

The state of a file region after a data mutation depends on the type of mutation, whether it succeeds or fails, and whether there are concurrent mutations.

If primary tells client that a write succeeded, and no other client is writing the same part of the file(serial), all readers will see the write. -> "defined"

If successful concurrent writes to the same part of a file, and they all succeed, all readers will see the same content, but maybe it will be a mix of the writes. -> "consistent" but "undefined"

    E.g. C1 writes "ab", C2 writes "xy", everyone might see "axyb".

If primary doesn't tell the client that the write succeeded, different readers may see different content, or none. -> "inconsistent"

### Inconsistency Scenario

- There are CO, P, S1, S2 for servers, C1, C2 for clients.
- C1 tries to read chunk. Got #v10 and P, S1, S2 from CO and S2 is the closest. cached.
- C1 reads data from S2.
- Suddenly, CO cannot reach S2.
- C2 writes something to the chunk and now it's #v11 in P, S1.
- C1 tries to read chunk. Since the location and version numbers are cached, reads #v10 from S2.

### Some Questions

What if a primary crashes?
  - Remove that chunkserver from all chunkhandle lists.
  - For each chunk for which it was primary,
        - wait for lease to expire,
        - grant lease to another chunkserver holding that chunk.

What is a lease?
  - Permission to act as primary for a given time (60 seconds).
  - Primary promises to stop acting as primary before lease expires.
  - Coordinator promises not to change primaries until after expiration.
  - Separate lease per actively written chunk.


## How to get stronger consistency

- atomic writes: either all replicas are updated, or none, even if failures.
- read sees latest write.
- all readers see the same content (assuming no writes).











