package main

import (
	"crypto/md5"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"
)

func GetRandomString(n int) string {
	str := "0123456789abcdefghijklmnopqrstuvwxyz"
	bytes := []byte(str)
	result := []byte{}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < n; i++ {
		result = append(result, bytes[r.Intn(len(bytes))])
	}
	return string(result)
}

func main() {
	fmt.Println("pid:", os.Getpid())
	tmpStr := GetRandomString(1024 * 1024 * 1024) // 1G
	bytes := []byte(tmpStr)
	for {
		var wg sync.WaitGroup
		for i := 0; i < 16; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()

				hasher := md5.New()
				hasher.Write(bytes)
				hasher.Sum(nil)
			}(i)
		}
		wg.Wait()
	}
}
