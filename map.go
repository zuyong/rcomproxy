package main

import (
	"bufio"
	"os"
	"strings"
)

func ParseMapFile(mapPath string) *map[string]string {
	var m map[string]string
	if mapPath == "" {
		return nil
	} else {
		file, err := os.Open(mapPath)
		if err != nil {
			logger.Fatal(err)
		}
		defer file.Close()
		m = make(map[string]string)
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			s := scanner.Text()
			ss := strings.Split(s, ",")
			if len(ss) == 2 && ss[0] != "" && ss[1] != "" {
				m[ss[0]] = ss[1]
			}
		}

		if err := scanner.Err(); err != nil {
			logger.Fatal(err)
		}

	}
	return &m
}
