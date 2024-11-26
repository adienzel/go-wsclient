package main

import (
	"math"
	"sort"
	"time"

	"go.uber.org/zap"
)

func calculateStatistics(server_port uint16, durations []time.Duration, logger *zap.SugaredLogger) {
	if len(durations) == 0 {
		logger.Error("No data to calculate statistics.")
		return
	}

	// Convert durations to float64 for calculations 
	values := make([]float64, len(durations))
	for i, d := range durations {
		values[i] = float64(d)
	}
	
	// Calculate average 
	var sum float64 = 0.0 
	for _, v := range values { 
		sum += v 
	}
	average := sum / float64(len(values))
	
	// Calculate median 
	sort.Float64s(values) 
	median := values[len(values) / 2]
	if len(values) % 2 == 0 { // if value is even we average the 2 middle one
		median = (values[len(values) / 2 - 1] + values[len(values) / 2]) / 2
	}
	// Calculate standard deviation 
	var variance float64 = 0.0
	for _, v := range values {
		variance += math.Pow((v - average), 2)
	}
	variance /= float64(len(values))
	stdDev := math.Sqrt(variance)
	
	// Calculate skewness 
	var skewness float64 = 0.0
	for _, v := range values {
		skewness += math.Pow((v-average) / stdDev, 3)
	}
	skewness /= float64(len(values))
	
	// Calculate percentiles 
	percentiles := []float64{75, 90, 95, 99}
	percentileValues := make(map[float64]float64)
	for _, p := range percentiles {
		k := int(math.Ceil(p / 100 * float64(len(values))))
		percentileValues[p] = values[k-1]
	}
	// Print statistics 
	logger.Info("Port   |Average     |Median      |SDV         |Skew        |75P         |90P         |95P         |99P        |")
	logger.Infof("%06d |%.6f |%.6f |%.6f |%.6f |%.6f |%.6f |%.6f |%.6f |", 
	              server_port,
				  average,
				  median,
				  stdDev,
				  skewness,
				  percentileValues[0],
				  percentileValues[1],
				  percentileValues[2],
				  percentileValues[3])
}
	
// Function to collect durations and pass them to the statistics collector 
func collectDurations(durationChannel chan time.Duration, statsChannel chan []time.Duration) {
	var durations []time.Duration
	for {
		select {
			case d := <-durationChannel: 
				durations = append(durations, d)
				statsChannel <- durations
		}
	}
}
// Function to periodically calculate statistics
func periodicStatistics(server_port uint16, statsChannel chan []time.Duration, seconds time.Duration, logger *zap.SugaredLogger) {
	ticker := time.NewTicker(seconds * time.Second) 
	for {
		<-ticker.C 
		durations := <-statsChannel
		calculateStatistics(server_port, durations, logger)
	}
}