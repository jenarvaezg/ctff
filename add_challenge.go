package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"sync"
)

func challengeFromInterface(m map[string]interface{}) (c Challenge) {
	c = Challenge{}
	c.Title = m["Title"].(string)
	c.Description = template.HTML(m["Description"].(string))
	c.Category = m["Category"].(string)
	c.MaxScore = int(m["MaxScore"].(float64))
	c.Creator = m["Creator"].(string)
	c.UID = getSha512Hex(c.Title)
	return
}

func readInfoJSON(dirname string) (m map[string]interface{}, err error) {
	f, err := os.Open(dirname + "/info.json")
	fmt.Println(dirname)
	if err != nil {
		return
	}
	content, err := ioutil.ReadAll(f)
	if err != nil {
		return
	}
	err = json.Unmarshal(content, &m)
	return
}

func challengeFromInfoJSON(dirname string) (c Challenge, err error) {
	m, err := readInfoJSON(dirname)
	if err != nil {
		return
	}
	c = challengeFromInterface(m)
	splitted := strings.Split(dirname, "/")
	c.Alias = splitted[len(splitted)-1]

	fmt.Println(c.Alias)
	return
}

func addNewChallenges() {
	fileInfos, err := ioutil.ReadDir(ChallengesPath)
	if err != nil {
		log.Fatal(err)
	}
	aliases := GetAllChallengeAliases()
	old_challenges := make(map[string]bool)
	for _, fileInfo := range fileInfos {
		if !fileInfo.IsDir() {
			continue
		}
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
			fmt.Println("GOT " + k)

			wg.Add(1)
			go func(alias string) {
				defer wg.Done()
				c, err := challengeFromInfoJSON(ChallengesPath + "/" + alias)
				if err != nil {
					log.Println(err)
					return
				}
				c.addChallenge()
			}(k)
		}
	}
	wg.Wait()
}

func extractChallenge(srcFile string) error {
	f, err := os.Open(srcFile)
	if err != nil {
		return errors.New("Opening file " + srcFile + ": " + err.Error())
	}
	defer f.Close()
	gzf, err := gzip.NewReader(f)
	if err != nil {
		return errors.New("Creating gzip reader: " + err.Error())
	}

	tarReader := tar.NewReader(gzf)
	for header, err := tarReader.Next(); err != io.EOF; header, err = tarReader.Next() {
		if err != nil {
			return errors.New("getting next: " + err.Error())
		}

		name := header.Name
		switch header.Typeflag {
		case tar.TypeDir:
			err := os.MkdirAll("/tmp/"+name, 0755)
			if err != nil {
				return err
			}
		case tar.TypeReg:
			fmt.Println("Name: ", name)
			f, err := os.Create("/tmp/" + name)
			if err != nil {
				f.Close()
				return err
			}
			f.Chmod(os.FileMode(header.Mode))
			io.Copy(f, tarReader)
			f.Close()
		default:
			fmt.Println(header.Typeflag, tar.TypeReg)
			return errors.New("Weird header: " + string(header.Typeflag) + " in file " + name)
		}

	}
	return nil
}

func installChallenges(challenges []string) {
	var wg sync.WaitGroup

	for _, challenge := range challenges {
		if !strings.HasSuffix(challenge, ".ctff") {
			fmt.Println(challenge, "needs to be a .ctff file")
			continue
		}
		wg.Add(1)
		go func(challenge string) {
			defer wg.Done()
			err := extractChallenge(challenge)
			if err != nil {
				fmt.Println(err)
				return
			}

			tmpDirName := "/tmp/" + challenge[:len(challenge)-len(".ctff")]
			c, err := challengeFromInfoJSON(tmpDirName)
			if err != nil {
				fmt.Println(err)
				return
			}
			if ChallengeExists(c.UID) {
				fmt.Println("CHALLENGE "+challenge, "ALREADY EXISTS")
				return
			}
			if err := c.addChallenge(); err != nil {
				log.Println(err)
				return
			}
			os.Rename(tmpDirName, ChallengesPath+"/"+c.Alias)

		}(challenge)
	}
	wg.Wait()

}
