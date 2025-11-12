package historydb

import (
	"bytes"
	"encoding/gob"
	"errors"
	"sync"
	"time"

	"github.com/coocood/freecache"
)

// seriesMutex global safe lock
var seriesMutex sync.Mutex

// TimeSeriesData provides a slice data struct
type TimeSeriesData struct {
	DataPoints [][2]any
}

// Serialize a series data
func (ts *TimeSeriesData) Serialize() ([]byte, error) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	err := encoder.Encode(ts)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// DeserializeSeries a series data
func DeserializeSeries(data []byte) (TimeSeriesData, error) {
	var ts TimeSeriesData
	buf := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(buf)
	err := decoder.Decode(&ts)
	return ts, err
}

// RecordDataPoint records a data point
func RecordDataPoint(key string, newPoint [2]any) error {
	// Prepare key
	cacheKey := []byte(key)

	// Try to get data
	existingData, err := C.Get(cacheKey)
	if err != nil && !errors.Is(err, freecache.ErrNotFound) {
		return err
	}

	// DeserializeAcc
	var timeSeries TimeSeriesData
	if err == nil {
		timeSeries, err = DeserializeSeries(existingData)
		if err != nil {
			return err
		}
	} else {
		timeSeries = TimeSeriesData{
			DataPoints: [][2]any{},
		}
	}

	// Append data
	timeSeries.DataPoints = append(timeSeries.DataPoints, newPoint)

	// Serialize new data
	serializedData, err := timeSeries.Serialize()
	if err != nil {
		return err
	}

	// Set to cache
	return C.Set(cacheKey, serializedData, 0)
}

// RecordDataPointSafe records a data point with lock
func RecordDataPointSafe(key string, newPoint [2]any) error {
	seriesMutex.Lock()
	defer seriesMutex.Unlock()
	return RecordDataPoint(key, newPoint)
}

// GetDataSlice gets data slice from DB
func GetDataSlice(key string) ([][2]any, error) {
	// Get data from DB
	existingData, err := C.Get([]byte(key))
	if err != nil {
		return make([][2]any, 0), err
	}

	// DeserializeAcc
	var timeSeries TimeSeriesData
	timeSeries, err = DeserializeSeries(existingData)
	if err != nil {
		return make([][2]any, 0), err
	}

	return timeSeries.DataPoints, nil
}

// ClearDataSlice clears expired data from slice
func ClearDataSlice(key string, TTL float64) error {
	// Get time now
	now := time.Now()

	// Prepare key
	cacheKey := []byte(key)

	// Try to get data
	existingData, err := C.Get(cacheKey)
	if err != nil {
		return err
	}

	// DeserializeAcc
	var timeSeries TimeSeriesData
	timeSeries, err = DeserializeSeries(existingData)
	if err != nil {
		return err
	}

	// Append data
	newDataPoints := make([][2]any, 0)
	for _, dataPoint := range timeSeries.DataPoints {
		if dataPoint[0].(float64)+TTL > float64(now.UnixNano())/1e9 {
			newDataPoints = append(newDataPoints, dataPoint)
		}
	}

	// Serialize new data
	newData := &TimeSeriesData{DataPoints: newDataPoints}
	serializedData, err := newData.Serialize()
	if err != nil {
		return err
	}

	// Set to cache
	return C.Set(cacheKey, serializedData, 0)
}
