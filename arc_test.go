package gcache

import (
	"fmt"
	"testing"
)

func buildARCache(size int) (Cache, error) {
	return New(size).
		ARC().
		EvictedFunc(evictedFuncForARC).
		Build()
}

func buildLoadingARCache(size int) (Cache, error) {
	return New(size).
		ARC().
		LoaderFunc(loader).
		EvictedFunc(evictedFuncForARC).
		Build()
}

func evictedFuncForARC(key, value interface{}) {
	fmt.Printf("[ARC] Key:%v Value:%v will be evicted.\n", key, value)
}

func TestARCGet(t *testing.T) {
	size := 1000
	gc, err := buildARCache(size)
	if err != nil {
		t.Error(err)
	}

	testSetCache(t, gc, size)
	testGetCache(t, gc, size)
}

func TestLoadingARCGet(t *testing.T) {
	size := 1000
	numbers := 1000
	gc, err := buildLoadingARCache(size)
	if err != nil {
		t.Error(err)
	}
	testGetCache(t, gc, numbers)
}
