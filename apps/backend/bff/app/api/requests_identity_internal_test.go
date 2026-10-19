package api

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

var (
	identifierPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)
	subjectPattern    = regexp.MustCompile(`^[A-Za-z0-9._:-]{0,128}$`)
)

func identifierCandidates() []string {
	candidates := []string{"", "\u00e9", "\U0001F600", "a\u00e9", "\xff", "a\n", " a", "a "}
	for c := range 256 {
		char := string([]byte{byte(c)})
		for _, length := range []int{1, 128, 129} {
			candidates = append(candidates,
				strings.Repeat(char, length),
				strings.Repeat("a", length-1)+char,
				char+strings.Repeat("a", length-1))
		}
	}
	return candidates
}

func TestIdentifiersAreJudgedAsThePatternOfTheContract(t *testing.T) {
	for _, text := range identifierCandidates() {
		if got, want := validIdentifier(text), identifierPattern.MatchString(text); got != want {
			t.Errorf("validIdentifier(%q) = %t, want %t", text, got, want)
		}
		if got, want := validSubject(text), subjectPattern.MatchString(text); got != want {
			t.Errorf("validSubject(%q) = %t, want %t", text, got, want)
		}
	}
}

func TestAReferenceIsTheObjectOfItsOneMember(t *testing.T) {
	for _, r := range []reference{
		{"organizationId", "org-1"},
		{"membershipId", strings.Repeat("Az09._:-", 16)},
		{"userId", ""},
		{"userId", `<a href="x">&` + "\u00e9\U0001F600\u2028\x7f"},
	} {
		got, err := json.Marshal(r)
		want, _ := json.Marshal(map[string]string{r.member: r.id})
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("Marshal(%+v) = %s, %v, want %s", r, got, err, want)
		}
	}
}
