package ourutil

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/golang/glog"
)

func Reportf(f string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, f+"\n", args...)
	glog.Infof(f, args...)
}

func Freportf(logFile io.Writer, f string, args ...interface{}) {
	fmt.Fprintf(logFile, f+"\n", args...)
	glog.Infof(f, args...)
}

func Prompt(text string) string {
	fmt.Fprintf(os.Stderr, "%s ", text)
	ans, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimSpace(ans)
}

func FirstN(s string, n int) string {
	if n > len(s) {
		n = len(s)
	}
	return s[:n]
}

// Returns a map from regexp capture group name to the corresponding matched
// string.
// A return value of nil indicates no match.
func FindNamedSubmatches(r *regexp.Regexp, s string) map[string]string {
	matches := r.FindStringSubmatch(s)
	if matches == nil {
		return nil
	}

	result := make(map[string]string)
	for i, name := range r.SubexpNames()[1:] {
		result[name] = matches[i+1]
	}
	return result
}
