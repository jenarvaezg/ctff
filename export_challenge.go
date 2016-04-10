package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"sync"
)

func handleError(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

func TarGzWrite(_path string, tw *tar.Writer, fi os.FileInfo) {
	fr, err := os.Open(_path)
	handleError(err)
	defer fr.Close()

	h := new(tar.Header)
	h.Name = _path[len(ChallengesPath+"/"):]
	h.Size = fi.Size()
	h.Mode = int64(fi.Mode())
	h.ModTime = fi.ModTime()
	h.Typeflag = tar.TypeReg

	err = tw.WriteHeader(h)
	handleError(err)

	_, err = io.Copy(tw, fr)
	handleError(err)
}

func IterDirectory(dirPath string, tw *tar.Writer) {
	dir, err := os.Open(dirPath)
	handleError(err)
	defer dir.Close()
	dirStat, err := os.Stat(dirPath)
	handleError(err)
	fis, err := dir.Readdir(0)
	handleError(err)
	h := new(tar.Header)
	h.Name = dirPath[len(ChallengesPath+"/"):]
	h.Mode = 0600
	h.ModTime = dirStat.ModTime()
	h.Typeflag = tar.TypeDir
	err = tw.WriteHeader(h)
	handleError(err)
	for _, fi := range fis {
		curPath := dirPath + "/" + fi.Name()
		if fi.IsDir() {
			IterDirectory(curPath, tw)
		} else {
			TarGzWrite(curPath, tw, fi)
		}
	}
}

func tarGz(outFilePath string, inPath string) {
	// file write
	fw, err := os.Create(outFilePath)
	handleError(err)
	defer fw.Close()

	// gzip write
	gw := gzip.NewWriter(fw)
	defer gw.Close()

	// tar write
	tw := tar.NewWriter(gw)
	defer tw.Close()

	IterDirectory(inPath, tw)
}

func generateJSON(challengeAlias string) error {
	c, err := getChallengeByAlias(challengeAlias)
	if err != nil {
		return errors.New("Generate json " + err.Error())
	}
	blob, err := json.Marshal(c)
	if err != nil {
		return errors.New("Marshal " + err.Error())
	}
	err = ioutil.WriteFile(ChallengesPath+"/"+challengeAlias+"/info.json", blob, 0644)
	if err != nil {
		return errors.New("Writing info.json " + err.Error())
	}
	return nil
}

func exportChallenges(aliases []string) {
	var wg sync.WaitGroup

	for _, alias := range aliases {
		wg.Add(1)
		go func(_alias string) {
			defer wg.Done()
			fmt.Println("Trying to export " + _alias)
			if _, err := getChallengeByAlias(_alias); err != nil {
				fmt.Println("Challenge:", _alias, "does not exist")
				return
			}
			targetFilePath := _alias + ".ctff"
			challengeDirPath := ChallengesPath + "/" + _alias
			if _, err := os.Stat(challengeDirPath + "/info.json"); os.IsNotExist(err) {
				generateJSON(_alias)
			}
			tarGz(targetFilePath, strings.TrimRight(challengeDirPath, "/"))
		}(alias)
	}
	wg.Wait()

}
