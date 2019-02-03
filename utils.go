package main

import (
	"time"
)

// Ints returns a unique subset of the int slice provided.
func UniqueInts(input []int) []int {
	u := make([]int, 0, len(input))
	m := make(map[int]bool)

	for _, val := range input {
		if _, ok := m[val]; !ok {
			m[val] = true
			u = append(u, val)
		}
	}

	return u
}

func Exists(array []string, toSearch string) bool {
	for _, elem := range array {
		if elem == toSearch {
			return true
		}
	}

	return false
}

func GetEpoch() int64 {
	return time.Now().UTC().Unix()
}
