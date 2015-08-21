// Reads logs from stdin and writes them to rotating log files.  Useful for log
// rotation for 12 factor apps that write their logs to stdout
// (http://12factor.net/logs).
//
// Usage:
// $ ./app | ./logwheel --log /var/log/app/log
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const version = "1.0.0"

func open(path string) *os.File {
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	return f
}

type oldestFirst []string

func timestampSuffix(s string) int {
	pieces := strings.Split(s, ".")
	if len(pieces) < 2 {
		panic(fmt.Errorf("Missing timestamp suffix: %s", s))
	}
	t, err := strconv.Atoi(pieces[len(pieces)-1])
	if err != nil {
		panic(err)
	}
	return t
}

func (s oldestFirst) Len() int      { return len(s) }
func (s oldestFirst) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s oldestFirst) Less(i, j int) bool {
	return timestampSuffix(s[i]) < timestampSuffix(s[j])
}

func rotate(f *os.File, path string, maxOldFiles int) *os.File {
	f.Close()

	now := time.Now()
	rotatedPath := fmt.Sprintf("%s.%d", path, now.UnixNano())
	os.Rename(path, rotatedPath)

	// Clean up old rotated files.
	globPattern := path + ".*"
	rotatedFiles, err := filepath.Glob(globPattern)
	if err != nil {
		panic(err)
	}
	sort.Sort(oldestFirst(rotatedFiles))
	if len(rotatedFiles) > maxOldFiles {
		for _, staleFile := range rotatedFiles[:len(rotatedFiles)-maxOldFiles] {
			os.Remove(staleFile)
		}
	}

	// Open a new file to write to.
	return open(path)
}

func main() {
	logPtr := flag.String("log", "",
		"Write logs to this path. Rotated files will have a timestamp suffix.")
	maxOldFilesPtr := flag.Int("max-old-files", 2,
		"Keep this many of the most recent rotated files and delete the others.")
	maxBytesPtr := flag.Int("max-bytes", 50*(1<<20),
		"Rotate log files at this size.")
	versionPtr := flag.Bool("version", false, "Print version.")
	flag.Parse()
	if *versionPtr {
		fmt.Println("github.com/jtconnor/logwheel version " + version)
		return
	}
	if *logPtr == "" {
		panic(fmt.Errorf("Requires --log"))
	}

	scanner := bufio.NewScanner(os.Stdin)
	f := open(*logPtr)
	bytesWritten := 0
	for scanner.Scan() {
		line := scanner.Text()
		if err := scanner.Err(); err != nil {
			panic(err)
		}

		if len(line) > *maxBytesPtr-1 {
			line = line[:*maxBytesPtr-1]
		}

		if bytesWritten+len(line)+1 > *maxBytesPtr {
			f = rotate(f, *logPtr, *maxOldFilesPtr)
			bytesWritten = 0
		}

		f.WriteString(line)
		f.WriteString("\n")
		bytesWritten += len(line) + 1
	}
}
