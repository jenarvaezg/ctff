package main

import (
	"fmt"
	"os"
	"sync"
)

func removeChallenges(aliases []string) {
	var wg sync.WaitGroup
	for _, alias := range aliases {
		wg.Add(1)
		go func(alias string) {
			defer wg.Done()
			c, err := getChallengeByAlias(alias)
			if err != nil {
				fmt.Println("Challenge", alias, " not found!")
				return
			}
			RemoveChallenge(c.UID)
			os.RemoveAll(ChallengesPath + "/" + alias)
		}(alias)

	}
	wg.Wait()
}
