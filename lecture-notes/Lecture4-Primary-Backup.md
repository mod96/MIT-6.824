# Lecture4. Primary-Backup Replication

## Failures

- Fail-stop failures (dealt with Primary-Backup)
  - failure = computer stops
  - Type of failures that cause the component of a system experiencing this type of failure stops operating.
  - Cut-off power, network problem, etc.
  - Cannot handle: logic bugs, configuration error(e.g. file incorrect), malicious(e.g. hacking)
  - Maybe handle: primary failure due to earthquake.

## Challenges

- Has primary really failed? 
  - e.g. network failure and primary is still working with client.
  - Split-brain system.
- Keep primary-backup in sync?
  - Apply changes in right order
  - Deal with non-deterministic (make every operations be deterministic)
  - Fail over 
    - what if primary was middle of operation, or was going to send packet to client?
    - what if some machines crash at once?


## Two Approaches

- State transfer
  - client talks to the primary
  - primary makes check point of it's state and sends to the backup

- Replicated state machine (**RSM**)
  - client talks to the primary
  - primary sends the `operation` to the backup
  - Level of operations to replicate
    - application-level operations (file append, write)
    - machine-level(processor level, computer level) operations - vm ft!


## Case Study: Vmware FT

By using exploit virtualization they made replication transparent. Appears to the client that server is a single machine. This is cool because it is VMware product, and can use it nowadays.

### Atomic test-and-set operation on the shared storage

The system uses a network disk server, shared by both primary and backup (the "shared disk" in Figure 1). That network disk server has a "test-and-set service". The test-and-set service maintains a flag that is initially set to false. If the primary or backup thinks the other server is dead, and thus that it should take over by itself, it first sends a test-and-set operation to the disk server. The server executes roughly this code:
```go
  test-and-set() {
    acquire_lock()
    if flag == true:
      release_lock()
      return false
    else:
      flag = true
      release_lock()
      return true
```
The primary (or backup) only takes over ("goes live") if test-and-set returns true.

The higher-level view is that, if the primary and backup lose network contact with each other, we want only one of them to go live. The danger is that, if both are up and the network has failed, both may go live and develop split brain. If only one of the primary or backup can talk to the disk server, then that server alone will go live. But what if both can talk to the disk server? Then the `network disk server acts as a tie-breaker`; test-and-set returns true only to the first call.

### Behave like a single machine

- What if time interrupts, non-deterministic things? 

Since hypervisor controls instructions, it can capture that things. All the sources of randomness are controlled by the hypervisor. For example, the application may use the current time, or a hardware cycle counter, or precise interrupt times as sources of randomness. In all three cases the hypervisor intercepts the the relevant instructions on both primary and backup and ensures they produce the same values.

- What if multi-threading lock? Same winner in the primary must be the winner at the secondary. 
  - FT doesn't cope with multi-processor guests (It's much more complex!)

In general, the results of software running on multiple processors depends on exactly how the instruction streams on the processors were interleaved. For FT to ensure that the backup stays in sync with the primary, it would have to cause the interleaving to be the same on both computers. This turns out to be hard: you can't easily find out what the interleaving is on the primary, and you can't easily control it on either machine.

A more recent VMware product replicates multi-processor guests, probably using a different technique (replicating memory snapshots rather than operations?).

- What is being sent to the backup?....?
  - Linux instructions(inc, dev, ...) are not sent. They just run theirselves.
  - Only some special has to happen at this possible of diversions.
  - Most instructions are deterministic so we don't have to manage it.
  - Non-deterministic things must be handled. e.g. interrupts

- Divergent things
  - Timer interrupts
    - While primary was executing deterministic instruction '100', interrupt arrived. It sends to backup through logging channel with something like `{100, Interrupt, data1}`.
    - After some time, while primary was executing deterministic instruction '200', interrupt arrived. It's sent too. `{200, Interrupt, data2}`.
    - How can backup know where can it be executed? Can it know and wait until instruction 200?
    - Backup just stays behind one message.
  - Non-deterministic(ND)
    - When FT finds that os will run ND command, it runs it and records the result, send to backup like `{ND, result}`.
    - When backup re-executes, since backup's FT has the message, result is the same.
  - Input packets

- Output Rule
  - What if failure were to happen immediately after the primary executed the output operation? (no 'output' logging was sent)
    - client might receive inconsistent result (My explanation: client sends 'inc' to primary, primary 'inc' to it's value, say 10, then becomes 11. And primary tries to send back that to the client. But primary dies, without sending packet arrive/output event. Then client timeout, retries to the backup which is now a new primary. It gets 'inc' command and 'inc' it's value 10, and returns client 11. client actually needed 12, since it tried twice.)
  - Rule: the primary VM may not send an output to the external world, until the backup VM has received and acknowledged the log entry associated with the operation producing the output. (Because of this, bandwidth performance is significantly reduced: Table 2)















































