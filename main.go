package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const version = "1.0.4"

func open(path string) (*bufio.Writer, *os.File, int64) {
	if fileInfo, err := os.Stat(path); err == nil {
		f, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND, 0666)
		Check(err, "Failed to open log file for appending")
		return bufio.NewWriter(f), f, fileInfo.Size()
	} else if os.IsNotExist(err) {
		f, err := os.Create(path)
		Check(err, "Failed to create output log file")
		return bufio.NewWriter(f), f, 0
	} else {
		panic(err)
	}
}

func close(w *bufio.Writer, f *os.File) {
	Check(w.Flush(), "Failed to flush output writer")
	Check(f.Close(), "Failed to close output log file")
}

type oldestFirst []string

func timestampSuffix(s string) int64 {
	pieces := strings.Split(s, ".")
	Assert(len(pieces) >= 2, fmt.Sprintf("Missing timestamp suffix: %s", s))
	t, err := strconv.ParseInt(pieces[len(pieces)-1], 10, 64)
	Check(err, fmt.Sprintf("Failed to parse timestamp suffix from: %s", s))
	return t
}

func (s oldestFirst) Len() int      { return len(s) }
func (s oldestFirst) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s oldestFirst) Less(i, j int) bool {
	return timestampSuffix(s[i]) < timestampSuffix(s[j])
}

func rotate(w *bufio.Writer, f *os.File, path string, maxOldFiles int) (
	*bufio.Writer, *os.File, int64) {
	close(w, f)

	now := time.Now()
	rotatedPath := fmt.Sprintf("%s.%d", path, now.UnixNano())
	os.Rename(path, rotatedPath)

	// Clean up old rotated files.
	globPattern := path + ".*"
	rotatedFiles, err := filepath.Glob(globPattern)
	Check(err, "Failed to look for old/rotated log files")
	sort.Sort(oldestFirst(rotatedFiles))
	if len(rotatedFiles) > maxOldFiles {
		for _, staleFile := range rotatedFiles[:len(rotatedFiles)-maxOldFiles] {
			os.Remove(staleFile)
		}
	}

	// Open a new file to write to.
	return open(path)
}

type VError struct {
	message string
	child   error
}

func (e VError) Error() string {
	return fmt.Sprintf("%s: %s", e.message, e.child.Error())
}

func NewVError(message string, child error) VError {
	return VError{message, child}
}

func Check(err error, message string) {
	if err != nil {
		panic(NewVError(message, err))
	}
}

func Assert(condition bool, message string) {
	if !condition {
		panic(errors.New(message))
	}
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
	Assert(*logPtr != "", "Requires --log")

	scanner := bufio.NewScanner(os.Stdin)
	w, f, bytesWritten := open(*logPtr)
	for scanner.Scan() {
		line := scanner.Text()
		Check(scanner.Err(), "Failed to scan input")

		lineLen := int64(len(line))

		if lineLen > *maxBytesPtr-1 {
			line = line[:*maxBytesPtr-1]
		}

		if bytesWritten+lineLen+1 > *maxBytesPtr {
			w, f, bytesWritten = rotate(w, f, *logPtr, *maxOldFilesPtr)
		}

		w.WriteString(line)
		w.WriteString("\n")
		bytesWritten += lineLen + 1
	}
	close(w, f)
}
