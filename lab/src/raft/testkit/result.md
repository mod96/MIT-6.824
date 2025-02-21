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

```bash
VERBOSE=1 go test -run TestBasicAgree2B
time go test -run 2B
```

```bash
python ./testkit/dstest.py -p 1 -n 1 -o ./testkit TestRejoin2B
python ./testkit/dslogs.py ./testkit/TestRejoin2B_0.log -c 3
```
