https://blog.josejg.com/debugging-pretty/

```
cd raft
source ./testkit/.venv/bin/activate
```


### 2A

```bash
go test -run 2A
```

```bash
python ./testkit/dstest.py -p 10 -n 50 -o ./testkit 2A
```

```
(.venv) ~$ time go test -run 2A
Test (2A): initial election ...
  ... Passed --   3.1  3   74   20116    0
Test (2A): election after network failure ...
  ... Passed --   4.5  3  158   29498    0
Test (2A): multiple elections ...
  ... Passed --   5.6  7  864  160506    0
PASS
ok      6.824/raft      13.175s

real    0m13.463s
user    0m0.608s
sys     0m0.439s
```

```
(.venv) ~$ python ./testkit/dstest.py -p 20 -n 500 -o ./testkit 2A
┏━━━━━━┳━━━━━━━━┳━━━━━━━┳━━━━━━━━━━━━━━┓
┃ Test ┃ Failed ┃ Total ┃         Time ┃
┡━━━━━━╇━━━━━━━━╇━━━━━━━╇━━━━━━━━━━━━━━┩
│ 2A   │      0 │   500 │ 13.27 ± 0.41 │
└──────┴────────┴───────┴──────────────┘
```

### 2B

some debugging history...
```bash
VERBOSE=1 go test -run TestBasicAgree2B
go test -run 2B
python ./testkit/dstest.py -p 1 -n 1 -o ./testkit TestBackup2B
python ./testkit/dslogs.py ./testkit/TestBackup2B_0.log -c 5
```

results

```
(.venv) ~$ time go test -run 2B
Test (2B): basic agreement ...
  ... Passed --   0.5  3   16    4384    3
Test (2B): RPC byte count ...
  ... Passed --   1.1  3   50  114150   11
Test (2B): agreement after follower reconnects ...
  ... Passed --   3.5  3  124   30271    7
Test (2B): no agreement if too many followers disconnect ...
  ... Passed --   3.3  5  347   67381    3
Test (2B): concurrent Start()s ...
  ... Passed --   0.6  3   16    4400    6
Test (2B): rejoin of partitioned leader ...
  ... Passed --   3.9  3  254   54635    4
Test (2B): leader backs up quickly over incorrect follower logs ...
  ... Passed --  13.1  5 2342 1393454  102
Test (2B): RPC counts aren't too high ...
  ... Passed --   2.0  3   52   14786   12
PASS
ok      6.824/raft      28.066s

real    0m27.633s
user    0m1.570s
sys     0m1.040s
```

```
(.venv) ~$ python ./testkit/dstest.py -p 20 -n 500 -o ./testkit 2B
┏━━━━━━┳━━━━━━━━┳━━━━━━━┳━━━━━━━━━━━━━━┓
┃ Test ┃ Failed ┃ Total ┃         Time ┃
┡━━━━━━╇━━━━━━━━╇━━━━━━━╇━━━━━━━━━━━━━━┩
│ 2B   │      0 │   500 │ 30.02 ± 1.35 │
└──────┴────────┴───────┴──────────────┘
```


### 2C

```
python ./testkit/dstest.py -p 20 -n 500 -o ./testkit TestFigure8Unreliable2C
python ./testkit/dslogs.py ./testkit/TestFigure8Unreliable2C_93.log -c 5
```

```
(.venv) ~$ time go test -run 2C
Test (2C): basic persistence ...
  ... Passed --   3.5  3  109   28435    6
Test (2C): more persistence ...
  ... Passed --  14.9  5 1359  284781   16
Test (2C): partitioned leader and one follower crash, leader restarts ...
  ... Passed --   1.4  3   43   11115    4
Test (2C): Figure 8 ...
  ... Passed --  31.9  5 1955  911823   69
Test (2C): unreliable agreement ...
  ... Passed --   1.7  5  409  129805  246
Test (2C): Figure 8 (unreliable) ...
  ... Passed --  33.9  5 6781 7125330  107
Test (2C): churn ...
  ... Passed --  16.4  5 8829 4212601 3469
Test (2C): unreliable churn ...
  ... Passed --  16.1  5 4147 2502926 1180
PASS
ok      6.824/raft      119.862s

real    2m0.132s
user    0m20.634s
sys     0m5.801s
```

```
(.venv) ~$ python ./testkit/dstest.py -p 20 -n 500 -o ./testkit 2C
┏━━━━━━┳━━━━━━━━┳━━━━━━━┳━━━━━━━━━━━━━━━┓
┃ Test ┃ Failed ┃ Total ┃          Time ┃
┡━━━━━━╇━━━━━━━━╇━━━━━━━╇━━━━━━━━━━━━━━━┩
│ 2C   │      0 │   500 │ 116.14 ± 3.96 │
└──────┴────────┴───────┴───────────────┘
```

### 2D

In this case, InstallSnapshot RPC NEEDS success/failure flag at reply since old snapshot install can fail repeatedly.

```
VERBOSE=1 go test -run TestSnapshotBasic2D > out
python ./testkit/dslogs.py ./out -c 3


VERBOSE=1 go test -run TestSnapshotInstall2D

python ./testkit/dstest.py -p 1 -n 1 -o ./testkit TestSnapshotInstall2D
python ./testkit/dslogs.py ./testkit/TestSnapshotInstall2D_0.log -c 3

python ./testkit/dstest.py -p 1 -n 1 -o ./testkit TestSnapshotInstallUnreliable2D
python ./testkit/dslogs.py ./testkit/TestSnapshotInstallUnreliable2D_6.log -c 3
```

This test requires crashed/partitioned follower to quickly catch up missed logs. So, sending logs in single threaded and relatively long timeout makes tests take long, more than 120s per test case.

```
(.venv) ~$ time go test -run 2D
Test (2D): snapshots basic ...
  ... Passed --   3.2  3  139   49477  235
Test (2D): install snapshots (disconnect) ...
  ... Passed --  49.0  3 1604  516969  328
Test (2D): install snapshots (disconnect+unreliable) ...
  ... Passed --  51.5  3 1749  520155  336
Test (2D): install snapshots (crash) ...
  ... Passed --  34.7  3 1100  390438  345
Test (2D): install snapshots (unreliable+crash) ...
  ... Passed --  40.0  3 1296  428715  326
Test (2D): crash and restart all servers ...
  ... Passed --   6.6  3  291   86235   62
PASS
ok      6.824/raft      185.019s

real    3m2.089s
user    0m2.275s
sys     0m0.902s
```

```
(.venv) ~$ python ./testkit/dstest.py -p 20 -n 500 -o ./testkit 2D
┏━━━━━━┳━━━━━━━━┳━━━━━━━┳━━━━━━━━━━━━━━━┓
┃ Test ┃ Failed ┃ Total ┃          Time ┃
┡━━━━━━╇━━━━━━━━╇━━━━━━━╇━━━━━━━━━━━━━━━┩
│ 2D   │      0 │   500 │ 193.27 ± 7.76 │
└──────┴────────┴───────┴───────────────┘
```
