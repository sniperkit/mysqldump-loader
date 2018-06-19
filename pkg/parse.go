package mysqldump_loader

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
)

func parseIdentifier(s string, i int, terms string) (string, int, error) {
	var buf bytes.Buffer
	if s[i] == '`' || s[i] == '"' {
		quote := s[i]
		i++
		for {
			j := strings.IndexByte(s[i:], quote)
			if j == -1 {
				return "", 0, fmt.Errorf("name is not enclosed by '%c'", quote)
			}
			buf.WriteString(s[i : i+j])
			i += j + 1
			if strings.IndexByte(terms, s[i]) != -1 {
				break
			} else if s[i] == quote {
				buf.WriteByte(quote)
			} else {
				return "", 0, fmt.Errorf("unexpected character '%c'", s[i])
			}
		}
	} else {
		j := strings.IndexAny(s[i:], terms)
		if j == -1 {
			return "", 0, errors.New("name is not terminated")
		} else {
			buf.WriteString(s[i : i+j])
			i += j
		}
	}
	return buf.String(), i, nil
}
