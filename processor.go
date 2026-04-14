package main

import (
	"bufio"
	"container/heap"
	"io"
	"strings"
	"sync"
)

// --- Токенизация и нормализация слов ---
func tokenize(text string) []string {
	text = strings.ToLower(text)
	splitText := strings.FieldsFunc(text, func(r rune) bool {
		return r == ',' || r == '.' || r == '!' || r == ' ' || r == '\n'
	})

	return splitText
}

// --- Map-фаза ---
func mapWords(chunk []byte) map[string]int {
	// Токенизация + нормализация + счёт
	wordFreq := make(map[string]int)

	rawData := string(chunk)

	data := tokenize(rawData)

	for _, value := range data {
		count, exist := wordFreq[value]
		if !exist {
			wordFreq[value] = 1
		} else {
			wordFreq[value] = count + 1
		}
	}
	return wordFreq
}

// --- Read-фаза ---
func readChunks(file io.Reader, chunkSize int, out chan<- []byte, wg *sync.WaitGroup) {
	// Чтение блоками и отправка в канал

	defer wg.Done()

	reader := bufio.NewReader(file)
	var remainder []byte
	buf := make([]byte, chunkSize)

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			data := append(remainder, buf[:n]...)
			// Ищем последний разделитель (пробел, пунктуация, новая строка)
			lastDelim := -1
			for i := len(data) - 1; i >= 0; i-- {
				c := data[i]
				if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
					lastDelim = i
					break
				}
			}
			if lastDelim != -1 {
				// Отправляем всё до разделителя включительно
				out <- data[:lastDelim+1]
				remainder = data[lastDelim+1:]
			} else {
				remainder = data
			}
		}
		if err != nil {
			if err == io.EOF {
				if len(remainder) > 0 {
					out <- remainder
				}
				break
			}
			break
		}
	}

}

// --- Reduce-фаза ---
func reduceWordCounts(in <-chan map[string]int, done chan<- map[string]int) {
	// Объединение результатов
	ensambleMap := make(map[string]int)

	for mapValue := range in {
		for key, value := range mapValue {
			count, exist := ensambleMap[key]
			if !exist {
				ensambleMap[key] = value
			}
			ensambleMap[key] = count + value
		}
	}

	done <- ensambleMap
}

// --- Heap ---
type WordFreq struct {
	Word  string
	Count int
}

type MinHeap []WordFreq

func (h MinHeap) Len() int           { return len(h) }
func (h MinHeap) Less(i, j int) bool { return h[i].Count < h[j].Count }
func (h MinHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *MinHeap) Push(x interface{}) {
	*h = append(*h, x.(WordFreq))
}

func (h *MinHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

func getTopWords(freqMap map[string]int, n int) []WordFreq {
	// Нахождение топ-N
	h := &MinHeap{}
	heap.Init(h)

	for word, count := range freqMap {

		if h.Len() < n {
			heap.Push(h, WordFreq{Word: word, Count: count})
		} else if count > (*h)[0].Count {
			heap.Pop(h)
			heap.Push(h, WordFreq{word, count})
		}
	}

	// Извлекаем элементы в порядке возрастания частоты, затем переворачиваем
	result := make([]WordFreq, h.Len())
	for i := len(result) - 1; i >= 0; i-- {
		result[i] = heap.Pop(h).(WordFreq)
	}
	return result
}

func ProcessFile(file io.ReadCloser, chunkSize int, topN int) ([]WordFreq, error) {
	defer file.Close()
	// Процессинг файла
	var wgRead sync.WaitGroup
	var wg sync.WaitGroup
	out := make(chan []byte)
	mapsIn := make(chan map[string]int)
	mapsDone := make(chan map[string]int)

	wgRead.Add(1)
	go readChunks(file, chunkSize, out, &wgRead)
	go func() {
		wgRead.Wait()
		close(out)
	}()

	// Запускаем воркеров mapWords
	workers := 5
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for chunk := range out {
				m := mapWords(chunk)
				mapsIn <- m
			}
		}()
	}

	go reduceWordCounts(mapsIn, mapsDone)

	go func() {
		wg.Wait()
		close(mapsIn)
	}()

	Finalmap := <-mapsDone

	words := getTopWords(Finalmap, topN)

	// вот тут та и нужена вэйтгруппа
	return words, nil
}
