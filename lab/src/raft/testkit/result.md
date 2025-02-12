https://blog.josejg.com/debugging-pretty/

```
source ./testkit/.venv/bin/activate
```


### 2A

```
python ./testkit/dstest.py -p 10 -n 50 -o ./testkit TestManyElections2A
python ./testkit/dslogs.py ./testkit/TestManyElections2A_4.log -c 7
```
