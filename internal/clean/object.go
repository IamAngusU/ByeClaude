package clean

import (
	"bytes"
	"fmt"
	"strings"
)

type header struct {
	Key   string
	Lines []string
}

type commitObject struct {
	Headers []header
	Message string
}

func parseCommit(raw []byte) (commitObject, error) {
	text := string(raw)
	sep := strings.Index(text, "\n\n")
	if sep < 0 {
		return commitObject{}, fmt.Errorf("invalid commit object: missing header separator")
	}
	headerText, msg := text[:sep], text[sep+2:]
	var headers []header
	for _, line := range strings.Split(headerText, "\n") {
		if strings.HasPrefix(line, " ") {
			if len(headers) == 0 {
				return commitObject{}, fmt.Errorf("invalid commit continuation")
			}
			headers[len(headers)-1].Lines = append(headers[len(headers)-1].Lines, line)
			continue
		}
		key := line
		if i := strings.IndexByte(line, ' '); i >= 0 {
			key = line[:i]
		}
		headers = append(headers, header{Key: key, Lines: []string{line}})
	}
	return commitObject{Headers: headers, Message: msg}, nil
}

func (c commitObject) parents() []string {
	var out []string
	for _, h := range c.Headers {
		if h.Key == "parent" && len(h.Lines) > 0 {
			p := strings.TrimSpace(strings.TrimPrefix(h.Lines[0], "parent "))
			out = append(out, p)
		}
	}
	return out
}

func (c commitObject) author() (string, string) {
	for _, h := range c.Headers {
		if h.Key != "author" || len(h.Lines) == 0 {
			continue
		}
		line := strings.TrimPrefix(h.Lines[0], "author ")
		lt, gt := strings.LastIndex(line, "<"), strings.LastIndex(line, ">")
		if lt >= 0 && gt > lt {
			return strings.TrimSpace(line[:lt]), strings.TrimSpace(line[lt+1 : gt])
		}
	}
	return "", ""
}

func rebuildCommit(c commitObject, parentMap map[string]string, newMessage string) (raw []byte, dropped int) {
	var b bytes.Buffer
	for _, h := range c.Headers {
		if h.Key == "gpgsig" || h.Key == "gpgsig-sha256" || h.Key == "mergetag" {
			dropped++
			continue
		}
		if h.Key == "parent" && len(h.Lines) > 0 {
			old := strings.TrimSpace(strings.TrimPrefix(h.Lines[0], "parent "))
			if n, ok := parentMap[old]; ok {
				fmt.Fprintf(&b, "parent %s\n", n)
				continue
			}
		}
		for _, line := range h.Lines {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	b.WriteByte('\n')
	b.WriteString(newMessage)
	return b.Bytes(), dropped
}
