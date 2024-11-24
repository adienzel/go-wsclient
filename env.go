package main

import (
	"fmt"
	"os"
	"strconv"
)

func lookupEnvString(env string, defaultParam string) string {
	param, found := os.LookupEnv(env)
	if !found {
		param = defaultParam
	}
	return param
}

func lookupEnvint64(env string, defaultParam int64) int64 {
	param, found := os.LookupEnv(env)
	if !found {
		param = strconv.FormatInt(int64(defaultParam), 10)
	}
	i, err := strconv.ParseInt(param, 10, 64)
	if err != nil {
		i = 1
	}
	return i
}

func lookupEnvFloat64(env string, defaultParam float64) float64 {
	param, found := os.LookupEnv(env)
	if !found {
		param = fmt.Sprintf("%f", defaultParam)
	}
	i, err := strconv.ParseFloat(param, 64)
	if err != nil {
		i = 1
	}
	return i
}
