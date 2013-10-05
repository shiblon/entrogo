package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"code.google.com/p/entrogo/mealy"
	"os"
	"strings"
)

var (
	noWrite bool
)

func init() {
	flag.BoolVar(&noWrite, "nowrite", false, "Set to just display stats.")
}

func TextFileToChannel(inName string) <-chan string {
	words := make(chan string)
	go func() {
		defer close(words)

		file, err := os.Open(inName)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		r := bufio.NewReader(file)
		line, err := r.ReadString('\n')
		for ; err == nil; line, err = r.ReadString('\n') {
			words <- strings.TrimSpace(strings.ToUpper(line))
		}
		if err != io.EOF {
			log.Fatal(err)
		}
	}()
	return words
}

func WriteMealy(outName string, m mealy.MealyMachine) {
	file, err := os.Create(outName)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	err = m.WriteTo(file)
	if err != nil {
		log.Fatal(err)
	}
}

func ReadMealy(inName string) mealy.MealyMachine {
	file, err := os.Open(inName)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	m, err := mealy.ReadFrom(file)
	if err != nil {
		log.Fatal(err)
	}
	return m
}

func AreChannelsEqual(a, b <-chan string) (bool, error) {
	av, aok := <-a
	bv, bok := <-b
	for av == bv && aok == bok && (aok || bok) {
		av, aok = <-a
		bv, bok = <-b
	}
	switch {
	case aok != bok:
		return false, errors.New(
			fmt.Sprintf("Channels different length: %v:%t %v:%t", av, aok, bv, bok))
	case av != bv:
		return false, errors.New(
			fmt.Sprintf("Channels have different values: %v %v", av, bv))
	}
	return true, nil
}

func ByteFromStringChannel(source <-chan string) <-chan []byte {
	out := make(chan []byte)
	go func() {
		defer close(out)
		for x := range source {
			out <- []byte(x)
		}
	}()
	return out
}

func StringFromByteChannel(source <-chan []byte) <-chan string {
	out := make(chan string)
	go func() {
		defer close(out)
		for x := range source {
			out <- string(x)
		}
	}()
	return out
}

func BitsNeeded(val int) int {
	needed := 0
	for ; val > 0; needed++ {
		val >>= 1
	}
	return needed
}

func main() {
	flag.Parse()

	inName := flag.Arg(0)
	outName := flag.Arg(1)

	fmt.Printf("Reading file '%s'...\n", inName)
	machine := mealy.FromChannel(
		ByteFromStringChannel(TextFileToChannel(inName)))

	fmt.Print("Comparing sources for equivalence...")
	equal, err := AreChannelsEqual(
		TextFileToChannel(inName),
		StringFromByteChannel(machine.AllSequences()))

	switch {
	case equal:
		fmt.Println("  EQUAL")
	default:
		fmt.Println("\n  NOT EQUAL:\n  ", err)
		log.Fatal(err)
	}

	fmt.Println("Statistics for compiled machine:")
	fmt.Printf("  Number of states: %d (%x) (%d bits)\n", len(machine), len(machine), BitsNeeded(len(machine)))
	unique := machine.UniqueTransitions()
	fmt.Printf("  Number of unique transitions: %d (%x) (%d bits)\n", unique, unique, BitsNeeded(unique))
	numTransitions := machine.TotalTransitions()
	fmt.Printf("  Total transitions: %d\n", numTransitions)
	maxTransitions := machine.MaxStateTransitions()
	fmt.Printf("  Max transitions per state: %d (%x) (%d bits)\n", maxTransitions, maxTransitions, BitsNeeded(maxTransitions))
	allTriggers := machine.AllTriggers()
	fmt.Printf("  Number of trigger values: %d (%x) (%d bits)\n", len(allTriggers), len(allTriggers), BitsNeeded(len(allTriggers)))

	fmt.Println("  Triggers:")
	for _, trigger := range allTriggers {
		fmt.Printf("    %08x\n", trigger)
	}

	if noWrite {
		fmt.Println("Not writing because of flag.")
	} else {
		fmt.Printf("Writing serialized machine to '%s'...\n", outName)
		WriteMealy(outName, machine)

		fmt.Printf("Reading serialized mcahine from '%s'...\n", outName)
		writtenMachine := ReadMealy(outName)

		fmt.Print("Comparing built machine to deserialized version...")
		equal, err = AreChannelsEqual(
			StringFromByteChannel(machine.AllSequences()),
			StringFromByteChannel(writtenMachine.AllSequences()))
		switch {
		case equal:
			fmt.Println("  EQUAL")
		default:
			fmt.Println("  NOT EQUAL:\n  ", err)
			log.Fatal(err)
		}
	}
}
