package historydb

import (
	"errors"
	"sync"
	"time"

	"github.com/coocood/freecache"
	"github.com/ghinknet/json"
)

// mutex global safe lock
var mutex sync.Mutex

// DataPoint provides a basic data point struct
type DataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     any       `json:"value"`
}

// TimeSeriesData provides a slice data struct
type TimeSeriesData struct {
	DataPoints []DataPoint
}

// Serialize a series data
func (ts *TimeSeriesData) Serialize() ([]byte, error) {
	return json.Marshal(ts)
}

// Deserialize a series data
func Deserialize(data []byte) (TimeSeriesData, error) {
	var ts TimeSeriesData
	err := json.Unmarshal(data, &ts)
	return ts, err
}

// RecordDataPoint records a data point
func RecordDataPoint(key string, newPoint DataPoint) error {
	// Prepare key
	cacheKey := []byte(key)

	// Try to get data
	existingData, err := C.Get(cacheKey)
	if err != nil && !errors.Is(err, freecache.ErrNotFound) {
		return err
	}

	// Deserialize
	var timeSeries TimeSeriesData
	if err == nil {
		timeSeries, err = Deserialize(existingData)
		if err != nil {
			return err
		}
	} else {
		timeSeries = TimeSeriesData{
			DataPoints: []DataPoint{},
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
func RecordDataPointSafe(key string, newPoint DataPoint) error {
	mutex.Lock()
	defer mutex.Unlock()
	return RecordDataPoint(key, newPoint)
}

// GetDataSlice gets data slice from DB
func GetDataSlice(key string) ([]DataPoint, error) {
	// Get data from DB
	existingData, err := C.Get([]byte(key))
	if err != nil {
		return make([]DataPoint, 0), err
	}

	// Deserialize
	var timeSeries TimeSeriesData
	timeSeries, err = Deserialize(existingData)
	if err != nil {
		return make([]DataPoint, 0), err
	}

	return timeSeries.DataPoints, nil
}

// ClearDataSlice clears expired data from slice
func ClearDataSlice(key string, TTL time.Duration) error {
	// Get time now
	now := time.Now()

	// Prepare key
	cacheKey := []byte(key)

	// Try to get data
	existingData, err := C.Get(cacheKey)
	if err != nil {
		return err
	}

	// Deserialize
	var timeSeries TimeSeriesData
	timeSeries, err = Deserialize(existingData)
	if err != nil {
		return err
	}

	// Append data
	newDataPoints := make([]DataPoint, 0)
	for _, dataPoint := range timeSeries.DataPoints {
		if dataPoint.Timestamp.Add(TTL).After(now) {
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
