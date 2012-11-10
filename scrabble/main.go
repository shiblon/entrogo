// vim: noet sw=8 sts=8 cc=
//
// TODO: http://www.n3labs.com/pdf/lexicon-squeeze.pdf
//
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/scanner"
)

type Index struct {
	WordList       []string
	MissingLetters map[string]string
}

type MatchedWord struct {
	Word string
	Match string
	Prefix string
	Suffix string
	Needed map[string]int
}

func (idx *Index) Build(in chan string) {
	idx.WordList = make([]string, 0, 180000)
	idx.MissingLetters = make(map[string]string, cap(idx.WordList)*10)

	var wordSlice []string

	for word := range in {
		idx.WordList = append(idx.WordList, word)
		wordSlice = strings.Split(word, "")
		for ci, ch := range wordSlice {
			curSlice := make([]string, len(wordSlice))
			copy(curSlice, wordSlice)
			curSlice[ci] = "."
			curWord := strings.Join(curSlice, "")
			idx.MissingLetters[curWord] += ch
		}
	}
	sort.Strings(idx.WordList)
}

// Parse a query string and return a list of constraint strings that can be used
// to find valid words (assuming an unlimited supply of arbitrary letters).
//
// 	All letters are converted to uppercase before proceeding.
//
// 	If the string contains [...], then that's the available letter list
// 	(they can be repeated, and "." means "blank".
//
// 	The rest of the query can be bounded on either or both sides by "|",
// 	meaning that we should only find words that are bounded in that way.
//
// 	Other than bounds markers, the syntax is "." for any letter, "X" (a
// 	letter) for a specific letter that must be there, and <MA.PING> (dot
// 	must be present) for a letter that has to form a legal word in the
// 	given "." spot.
func ParseQuery(query string) (constraints []string) {
	// Query strings are themselves basically describable with a repeated
	// regular expression, where | is only allowed at the beginning or end
	// of the expression:
	pieceExp := `([.|[:alpha:]])|<([[:alpha:]]*?[.][[:alpha:]]*?)>`
	validExp := `^[|]?(([.[:alpha:]])|(<[[:alpha:]]*?[.][[:alpha:]]*?>))+[|]?$`
	validator, err := regexp.Compile(validExp)
	if err != nil {
		fmt.Printf("Error compiling regex %v: %v", validExp, err)
		return
	}
	piecer, err := regexp.Compile(pieceExp)
	if err != nil {
		fmt.Printf("Error compiling regex %v: %v", pieceExp, err)
		return
	}

	if !validator.MatchString(query) {
		fmt.Println("Query is incorrect: %s", query)
		return
	}

	pieces := piecer.FindAllStringSubmatch(query, -1)
	constraints = make([]string, 0, len(pieces))
	for _, groups := range pieces {
		var piece = groups[1]
		if piece == "" {
			piece = groups[2]
		}
		piece = strings.ToUpper(piece)
		constraints = append(constraints, piece)
	}
	return
}

func WordListFromReader(src io.Reader, out chan string) {
	var s scanner.Scanner
	s.Init(src)
	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		out <- s.TokenText()
	}
	close(out)
}

func main() {
	available := make(map[string]int)
	query := os.Args[1]
	if len(os.Args) > 2 {
		query = os.Args[2]
		for _, ch := range strings.Split(os.Args[1], "") {
			ch = strings.ToUpper(ch)
			available[ch]++
		}
	}
	index := Index{}

	queryPieces := ParseQuery(query)
	if len(queryPieces) == 0 {
		fmt.Println("Query could not be parsed. Quitting.")
		return
	}

	reader, err := os.Open("TWL06.txt")
	defer reader.Close()
	if err != nil {
		fmt.Printf(err.Error())
		return
	}
	words := make(chan string)
	go WordListFromReader(reader, words)

	fmt.Println("Building index.")
	index.Build(words)
	fmt.Printf("Index built for %d words with %d variations.\n",
		len(index.WordList), len(index.MissingLetters))

	// Now for each entry, find a list of letters that can work. Note that
	// we don't test for non-replacement, here. If there is a '.' in the
	// group, we'll get all letters. That may need to be optimized later,
	// but it doesn't seem super likely. The next pass, over actually
	// discovered words, will eliminate things based on replaceability.
	allowed := make([]string, len(queryPieces))
	drawIndices := make(map[int]bool, 0)
	possible := true
	curIndex := 0
	for i, qp := range queryPieces {
		switch {
		case qp == "|":
			if i == 0 {
				allowed[i] = "^"
			} else {
				allowed[i] = "$"
			}
			// Skip incrementing letter indices.
			continue
		case qp == ".":
			allowed[i] = qp
			drawIndices[curIndex] = true
		case len(qp) == 1:
			allowed[i] = qp
		default:
			found, ok := index.MissingLetters[qp]
			if ok {
				allowed[i] = found
				drawIndices[curIndex] = true
			} else {
				allowed[i] = "~"
				possible = false
			}
		}
		curIndex++
	}

	// Now actually search the word list for words that correspond to this,
	// using a regular expression.
	clauses := make([]string, len(allowed))
	for i, s := range allowed {
		if len(s) == 1 {
			clauses[i] = s
		} else {
			clauses[i] = fmt.Sprintf("[%v]", s)
		}
	}
	clauseStr := strings.Join(clauses, "")
	allowedExp, err := regexp.Compile(clauseStr)
	if err != nil {
		fmt.Printf("Failed to parse expression %v\n", clauseStr)
		return
	}
	if !possible {
		fmt.Printf("Impossible because of nearby letter constraints at '~': %v.\n", clauseStr)
		return
	}
	fmt.Printf("Match Expression: %v\n", allowedExp)

	matchedWords := []MatchedWord{}
	for _, word := range index.WordList {
		loc := allowedExp.FindStringIndex(word)
		if loc == nil {
			continue
		}
		// Figure out a comprehensive list of needed letters from the
		// prefix, suffix, and draw indices.
		m := MatchedWord{
			Word: word,
			Match: word[loc[0]:loc[1]],
			Prefix: word[:loc[0]],
			Suffix: word[loc[1]:],
		}
		needed := make(map[string]int, len(m.Prefix) + len(m.Suffix) + len(drawIndices))
		for ci, ch := range m.Match {
			if drawIndices[ci] {
				needed[string(ch)]++
			}
		}
		for _, ch := range m.Prefix {
			needed[string(ch)]++
		}
		for _, ch := range m.Suffix {
			needed[string(ch)]++
		}
		m.Needed = needed

		matchedWords = append(matchedWords, m)
	}
	if len(matchedWords) == 0 {
		fmt.Println("No words matched.")
		return
	}

	// Now we try to assign from our pool of possible letters, to each of
	// the not-fully-constrained bits in our words, skipping over the
	// endpoint notifiers. The drawIndices indicate which entries are used
	// to draw from our available letters.

	if len(available) == 0 {
		for _, w := range matchedWords {
			fmt.Println(w)
		}
		return
	}

	// We have a limited supply - use only what we have, favoring specific
	// letters over blank tiles first.
	workingWords := make([]MatchedWord, 0)
	for _, w := range matchedWords {
		left := make(map[string]int, len(available))
		for k, v := range(available) {
			left[k] = v
		}
		works := true
		for letter, needed := range w.Needed {
			remaining := left[letter] - needed
			if remaining < 0 {
				left["."] += remaining
				if left["."] < 0 {
					// We ran out of letters and blanks
					works = false
					break
				}
			}
		}
		if works {
			workingWords = append(workingWords, w)
		}
	}
	if len(workingWords) == 0 {
		fmt.Println("No words worked with your available tiles.")
		return
	}
	for _, w := range workingWords {
		fmt.Println(w.Word)
	}
}
