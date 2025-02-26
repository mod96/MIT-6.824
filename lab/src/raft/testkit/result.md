https://blog.josejg.com/debugging-pretty/

```
source ./testkit/.venv/bin/activate
python ./testkit/dslogs.py ./testkit/TestBasicAgree2B_10.log -c 3
```


### 2A

```bash
go test -run 2A
```

```bash
python ./testkit/dstest.py -p 10 -n 50 -o ./testkit 2A
```

```
(.venv) ~$ go test -run 2A
Test (2A): initial election ...
  ... Passed --   3.1  3   54   13422    0
Test (2A): election after network failure ...
  ... Passed --   4.4  3  118   20744    0
Test (2A): multiple elections ...
  ... Passed --   5.4  7  552   95124    0
PASS
ok      6.824/raft      12.929s
(.venv) ~$ python ./testkit/dstest.py -p 10 -n 50 -o ./testkit 2A
 Verbosity level set to 1
┏━━━━━━┳━━━━━━━━┳━━━━━━━┳━━━━━━━━━━━━━━┓
┃ Test ┃ Failed ┃ Total ┃         Time ┃
┡━━━━━━╇━━━━━━━━╇━━━━━━━╇━━━━━━━━━━━━━━┩
│ 2A   │      0 │    50 │ 13.36 ± 0.32 │
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
  ... Passed --   0.6  3   18    4312    3
Test (2B): RPC byte count ...
  ... Passed --   1.4  3   50  112960   11
Test (2B): agreement after follower reconnects ...
  ... Passed --   6.3  3  141   33802    8
Test (2B): no agreement if too many followers disconnect ...
  ... Passed --   3.5  5  224   42724    3
Test (2B): concurrent Start()s ...
  ... Passed --   1.0  3   24    5274    6
Test (2B): rejoin of partitioned leader ...
  ... Passed --   5.7  3  185   40383    4
Test (2B): leader backs up quickly over incorrect follower logs ...
  ... Passed --  16.6  5 1596  369713  103
Test (2B): RPC counts aren't too high ...
  ... Passed --   2.1  3   42   10912   12
PASS
ok      6.824/raft      37.351s

real    0m37.635s
user    0m1.185s
sys     0m0.567s
```

```
(.venv) ~$ python ./testkit/dstest.py -p 10 -n 50 -o ./testkit 2B
 Verbosity level set to 1
┏━━━━━━┳━━━━━━━━┳━━━━━━━┳━━━━━━━━━━━━━━┓
┃ Test ┃ Failed ┃ Total ┃         Time ┃
┡━━━━━━╇━━━━━━━━╇━━━━━━━╇━━━━━━━━━━━━━━┩
│ 2B   │      0 │    50 │ 39.66 ± 3.31 │
└──────┴────────┴───────┴──────────────┘
```


### 2C

```
(.venv) ~$ time go test -run 2C
Test (2C): basic persistence ...
  ... Passed --  12.9  3  275   65943    7
Test (2C): more persistence ...
  ... Passed --  32.8  5 1693  364561   23
Test (2C): partitioned leader and one follower crash, leader restarts ...
  ... Passed --   1.6  3   41    9167    4
Test (2C): Figure 8 ...
  ... Passed --  31.3  5 1345  414273   61
Test (2C): unreliable agreement ...
  ... Passed --   1.7  5  338   97485  246
Test (2C): Figure 8 (unreliable) ...
  ... Passed --  29.0  5 2926  691541  226
Test (2C): churn ...
  ... Passed --  16.1  5 8809 2805931 3616
Test (2C): unreliable churn ...
  ... Passed --  16.3  5 1118  254066  209
PASS
ok      6.824/raft      141.868s

real    2m22.120s
user    0m12.444s
sys     0m1.806s
```

```
(.venv) ~$ python ./testkit/dstest.py -p 10 -n 50 -o ./testkit 2C
 Verbosity level set to 1
┏━━━━━━┳━━━━━━━━┳━━━━━━━┳━━━━━━━━━━━━━━━┓
┃ Test ┃ Failed ┃ Total ┃          Time ┃
┡━━━━━━╇━━━━━━━━╇━━━━━━━╇━━━━━━━━━━━━━━━┩
│ 2C   │      0 │    50 │ 151.05 ± 7.34 │
└──────┴────────┴───────┴───────────────┘
```
