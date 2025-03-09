```go
package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

func main() {
	for i := 0; i < 10; i++ {
		count := 0
		mu := &sync.Mutex{}
		slice := make([]int, 0)
		for server := 0; server < 10; server++ {
			go func(trial int, sv int, count *int) {
				for i := 0; i < 10; i++ {
					mu.Lock()
					*count += 1
					fmt.Printf("[%d] server %d count %d\n", trial, sv, *count)
					time.Sleep(time.Duration(rand.Intn(50)+50) * time.Millisecond)
					slice = append(slice, *count)
					if len(slice) == 100 {
						fmt.Println(slice)
					}
					mu.Unlock()
				}
			}(i, server, &count)
		}
	}
	time.Sleep(100 * time.Second)
}
```

```go
package main

import (
	"fmt"
	"sync"
	"time"
)

func main() {
	var globalWG sync.WaitGroup
	for i := 0; i < 2; i++ {
		count := 0
		mu := &sync.Mutex{}
		wg := &sync.WaitGroup{}
		slice := make([]int, 0)
		for server := 0; server < 5; server++ {
			wg.Add(1)
			globalWG.Add(1)
			go func(trial int, sv int, count *int) {
				defer wg.Done()
				defer globalWG.Done()
				time.Sleep(time.Duration(2-trial) * time.Second)
				mu.Lock()
				*count += 1
				fmt.Printf("[T%d] server %d count %d\n", trial, sv, *count)
				slice = append(slice, *count)
				mu.Unlock()
			}(i, server, &count)
		}
		go func(trial int) {
			wg.Wait()
			fmt.Printf("[T%d] slice %v\n", trial, slice)
		}(i)
	}
	globalWG.Wait()
}
```