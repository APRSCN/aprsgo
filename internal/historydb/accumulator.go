package historydb

import (
	"bytes"
	"encoding/gob"
	"errors"
	"sync"

	"github.com/coocood/freecache"
)

// accMutex global safe lock
var accMutex sync.Mutex

// AccumulatorData provides an accumulator data struct
type AccumulatorData float64

// Serialize an accumulator data
func (acc *AccumulatorData) Serialize() ([]byte, error) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	err := encoder.Encode(acc)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// DeserializeAcc deserializes an accumulator data
func DeserializeAcc(data []byte) (AccumulatorData, error) {
	var acc AccumulatorData
	buf := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(buf)
	err := decoder.Decode(&acc)
	return acc, err
}

// IncrementValue increments the accumulator value
func IncrementValue(key string, increment float64) error {
	// Prepare key
	cacheKey := []byte(key)

	// Try to get data
	existingData, err := C.Get(cacheKey)
	if err != nil && !errors.Is(err, freecache.ErrNotFound) {
		return err
	}

	// DeserializeSeries or initialize
	var accumulator AccumulatorData
	if err == nil {
		accumulator, err = DeserializeAcc(existingData)
		if err != nil {
			return err
		}
	} else {
		accumulator = 0
	}

	// Increment value
	accumulator += AccumulatorData(increment)

	// Serialize new data
	serializedData, err := accumulator.Serialize()
	if err != nil {
		return err
	}

	// Set to cache
	return C.Set(cacheKey, serializedData, 0)
}

// IncrementValueSafe increments the accumulator value with lock
func IncrementValueSafe(key string, increment float64) error {
	accMutex.Lock()
	defer accMutex.Unlock()
	return IncrementValue(key, increment)
}

// DecrementValue decrements the accumulator value
func DecrementValue(key string, decrement float64) error {
	return IncrementValue(key, -decrement)
}

// DecrementValueSafe decrements the accumulator value with lock
func DecrementValueSafe(key string, decrement float64) error {
	accMutex.Lock()
	defer accMutex.Unlock()
	return DecrementValue(key, decrement)
}

// SetValue sets the accumulator to a specific value
func SetValue(key string, value float64) error {
	// Prepare key
	cacheKey := []byte(key)

	// Create new accumulator data
	accumulator := AccumulatorData(value)

	// Serialize data
	serializedData, err := accumulator.Serialize()
	if err != nil {
		return err
	}

	// Set to cache
	return C.Set(cacheKey, serializedData, 0)
}

// SetValueSafe sets the accumulator to a specific value with lock
func SetValueSafe(key string, value float64) error {
	accMutex.Lock()
	defer accMutex.Unlock()
	return SetValue(key, value)
}

// GetValue gets the current accumulator value
func GetValue(key string) (float64, error) {
	// Get data from DB
	existingData, err := C.Get([]byte(key))
	if err != nil {
		return 0, err
	}

	// DeserializeSeries
	var accumulator AccumulatorData
	accumulator, err = DeserializeAcc(existingData)
	if err != nil {
		return 0, err
	}

	return float64(accumulator), nil
}

// ResetValue resets the accumulator to zero
func ResetValue(key string) error {
	return SetValue(key, 0)
}

// ResetValueSafe resets the accumulator to zero with lock
func ResetValueSafe(key string) error {
	accMutex.Lock()
	defer accMutex.Unlock()
	return ResetValue(key)
}
