package main

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
)

func TestProcessFile(t *testing.T) {
	input := `Go go Go!
    Goroutine channel mutex map reduce.
    Go channel map.`

	r := io.NopCloser(strings.NewReader(input))
	top, err := ProcessFile(r, 32, 5)
	if err != nil {
		t.Fatalf("ProcessFile failed: %v", err)
	}

	expected := map[string]int{
		"go":        4,
		"channel":   2,
		"map":       2,
		"goroutine": 1,
		"mutex":     1,
	}

	fmt.Println(top)

	for _, wf := range top {
		if count, ok := expected[wf.Word]; !ok || count != wf.Count {
			t.Errorf("unexpected word %q with count %d", wf.Word, wf.Count)
		}
	}
}

func TestGetTopWords(t *testing.T) {
	freq := map[string]int{
		"go":        100,
		"goroutine": 50,
		"channel":   30,
		"mutex":     20,
		"heap":      15,
		"map":       10,
		"reduce":    5,
		"sync":      2,
		"waitgroup": 1,
		"cond":      1,
		"select":    1,
	}
	top := getTopWords(freq, 5)

	expected := []WordFreq{
		{"go", 100},
		{"goroutine", 50},
		{"channel", 30},
		{"mutex", 20},
		{"heap", 15},
	}

	if len(top) != len(expected) {
		t.Fatalf("expected %d elements, got %d", len(expected), len(top))
	}

	for i, wf := range top {
		if wf != expected[i] {
			t.Errorf("expected %v at position %d, got %v", expected[i], i, wf)
		}
	}
}

func TestReduceWordCounts(t *testing.T) {
	input := make(chan map[string]int, 2)
	output := make(chan map[string]int, 1)

	input <- map[string]int{"go": 1, "channel": 2}
	input <- map[string]int{"go": 3, "mutex": 1}
	close(input)

	reduceWordCounts(input, output)
	result := <-output

	expected := map[string]int{"go": 4, "channel": 2, "mutex": 1}

	if len(result) != len(expected) {
		t.Fatalf("expected %d keys, got %d", len(expected), len(result))
	}

	for k, v := range expected {
		if result[k] != v {
			t.Errorf("expected %s = %d, got %d", k, v, result[k])
		}
	}
}

func TestReadChunks(t *testing.T) {
	input := "word1 word2 word3 word4 word5"
	reader := strings.NewReader(input)
	out := make(chan []byte, 4)
	var wg sync.WaitGroup
	wg.Add(1)

	go readChunks(reader, 10, out, &wg)
	go func() {
		wg.Wait()
		close(out)
	}()

	total := 0
	for chunk := range out {
		total += len(chunk)
	}

	if total != len(input) {
		t.Errorf("expected to read %d bytes, read %d", len(input), total)
	}
}
