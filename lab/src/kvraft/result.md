```
cd kvraft
```


### 3A

```
VERBOSE=1 go test -run TestBasic3A
VERBOSE=1 go test -run TestUnreliable3A
```

```
~/w/m/MIT-6.824/la/src/kvraft main !2 ❯ go test -run 3A --race
Test: one client (3A) ...
  ... Passed --  15.0  5  8907 2532
Test: ops complete fast enough (3A) ...
  ... Passed --   9.4  3  3447    0
Test: many clients (3A) ...
  ... Passed --  15.1  5  9198 2556
Test: unreliable net, many clients (3A) ...
  ... Passed --  15.6  5  6134 1353
Test: concurrent append to same key, unreliable (3A) ...
  ... Passed --   1.3  3   231   52
Test: progress in majority (3A) ...
=> this is blocked somehow. need debugging
```
