package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"sync"
)

func challengeFromInterface(m map[string]interface{}) (c Challenge) {
	c = Challenge{}
	c.Title = m["Title"].(string)
	c.Description = template.HTML(m["Description"].(string))
	c.Category = m["Category"].(string)
	c.MaxScore = int(m["MaxScore"].(float64))
	c.Creator = m["Creator"].(string)
	return
}

func addChallengeSetup(wg *sync.WaitGroup, dirname string) {
	defer wg.Done()
	f, err := os.Open(dirname + "/info.json")
	if err != nil {
		fmt.Println(err)
		return
	}
	content, err := ioutil.ReadAll(f)
	if err != nil {
		fmt.Println(err)
		return
	}
	var m map[string]interface{}
	if err = json.Unmarshal(content, &m); err != nil {
		fmt.Println(err)
		return
	}
	c := challengeFromInterface(m)
	c.Alias = dirname[len(ChallengesPath+"/"):]
	fmt.Println(c)
	//addChallenge(c)
	c.Id = 0
	if err = c.AddToEnvironment(); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Now you have to add your code to the executables")
	defer f.Close()
}

func addNewChallenges() {
	fileInfos, err := ioutil.ReadDir(ChallengesPath)
	if err != nil {
		log.Fatal(err)
	}
	aliases := GetAllChallengeAliases()
	old_challenges := make(map[string]bool)
	for _, fileInfo := range fileInfos {
		old_challenges[fileInfo.Name()] = false
		for _, alias := range aliases {
			if fileInfo.Name() == alias {
				old_challenges[alias] = true
				break
			}
		}
	}
	var wg sync.WaitGroup
	for k, v := range old_challenges {
		if !v {
			wg.Add(1)
			fmt.Println("GOT " + k)
			go addChallengeSetup(&wg, ChallengesPath+"/"+k)
		}
	}
	wg.Wait()
}
