```
cd kvraft
```


### 3A

```
VERBOSE=1 go test -run TestBasic3A
VERBOSE=1 go test -run TestUnreliable3A
VERBOSE=1 go test -run TestManyPartitionsManyClients3A
VERBOSE=1 go test -run TestPersistPartition3A
```

```
~/w/m/MIT-6.824/la/src/kvraft main !2 ❯ go test -run 3A --race
Test: one client (3A) ...
  ... Passed --  15.1  5 39131 7553
Test: ops complete fast enough (3A) ...
  ... Passed --   0.9  3  3033    0
Test: many clients (3A) ...
  ... Passed --  15.1  5 38053 7329
Test: unreliable net, many clients (3A) ...
  ... Passed --  16.6  5  7398  860
Test: concurrent append to same key, unreliable (3A) ...
  ... Passed --   1.3  3   238   52
Test: progress in majority (3A) ...
  ... Passed --   0.6  5    95    2
Test: no progress in minority (3A) ...
  ... Passed --   1.0  5   286    3
Test: completion after heal (3A) ...
  ... Passed --   1.0  5    68    3
Test: partitions, one client (3A) ...
  ... Passed --  22.5  5 30194 7238
Test: partitions, many clients (3A) ...
  ... Passed --  24.2  5 32522 7456
Test: restarts, one client (3A) ...
  ... Passed --  19.7  5 40565 7806
Test: restarts, many clients (3A) ...
  ... Passed --  19.8  5 39393 7505
Test: unreliable net, restarts, many clients (3A) ...
  ... Passed --  20.5  5  7512  856
Test: restarts, partitions, many clients (3A) ...
  ... Passed --  27.1  5 32571 7523
Test: unreliable net, restarts, partitions, many clients (3A) ...
  ... Passed --  28.1  5  6603  523
Test: unreliable net, restarts, partitions, random keys, many clients (3A) ...
```
