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

const version = "1.0.1"

func open(path string) (*os.File, int64) {
	if fileInfo, err := os.Stat(path); err == nil {
		f, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND, 0666)
		if err != nil {
			panic(err)
		}
		return f, fileInfo.Size()
	} else if os.IsNotExist(err) {
		f, err := os.Create(path)
		if err != nil {
			panic(err)
		}
		return f, 0
	} else {
		panic(err)
	}
}

type oldestFirst []string

func timestampSuffix(s string) int64 {
	pieces := strings.Split(s, ".")
	if len(pieces) < 2 {
		panic(fmt.Errorf("Missing timestamp suffix: %s", s))
	}
	t, err := strconv.ParseInt(pieces[len(pieces)-1], 10, 64)
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

func rotate(f *os.File, path string, maxOldFiles int) (*os.File, int64) {
	if err := f.Close(); err != nil {
		panic(err)
	}

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
	maxBytesPtr := flag.Int64("max-bytes", 50*(1<<20),
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
	f, bytesWritten := open(*logPtr)
	for scanner.Scan() {
		line := scanner.Text()
		if err := scanner.Err(); err != nil {
			panic(err)
		}

		lineLen := int64(len(line))

		if lineLen > *maxBytesPtr-1 {
			line = line[:*maxBytesPtr-1]
		}

		if bytesWritten+lineLen+1 > *maxBytesPtr {
			f, bytesWritten = rotate(f, *logPtr, *maxOldFilesPtr)
		}

		f.WriteString(line)
		f.WriteString("\n")
		bytesWritten += lineLen + 1
	}
	if err := f.Close(); err != nil {
		panic(err)
	}
}
