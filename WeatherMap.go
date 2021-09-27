package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	
	"sync"
	"time"

	ini "gopkg.in/ini.v1"
)

// struct of Wind_Speed and Temperature. 
type Weatherresult struct {
	
	Wind_speed          int
	Temperature_degrees int
	
}


// List of Global Variables 
var primaryurl string
var backupurl string
var currenturl string
var cachedweather Weatherresult
var clearedcache Weatherresult
var mutex sync.Mutex
var mutexforcache sync.Mutex


//Start of main() functions.
func main() {

	cfg, err := ini.Load("env.ini")
	if err != nil {
		log.Println("Fail to read file:", err)
		os.Exit(1)
	}

	primaryurl = "http://api.weatherstack.com/current?access_key=" + cfg.Section("API").Key("WEATHERSTACK").String() + "&query=Melbourne"

	backupurl = "http://api.openweathermap.org/data/2.5/weather?q=melbourne,AU&appid=" + cfg.Section("API").Key("OPENWEATHER").String()

	currenturl = primaryurl

	http.HandleFunc("/v1/weather", Handler)
	fmt.Println("Started Listening....")
	log.Fatal(http.ListenAndServe("localhost:8080", nil))

}

// Go routines for Checing whether the Primary URL is up.
func WithGo() {
	for {
		_, err := http.Get(primaryurl)

		if err == nil {
			mutex.Lock()
			currenturl = primaryurl
			mutex.Unlock()
			break
		}
		time.Sleep(5 * time.Second)

	}

}

// Go routines for clearing the caches. 
func ClearingCache() {

	time.Sleep(10 * time.Second)
	cachedweather = clearedcache
	mutexforcache.Lock()
	clearedcache = Weatherresult{}
	mutexforcache.Unlock()

}


// Manipulating the HTTP requests and its response. 
func Handler(w http.ResponseWriter, r *http.Request) {

	if (Weatherresult{}) != clearedcache {
		fmt.Println("Cache Hit !")
		convertojson, err := json.Marshal(clearedcache)

		if err != nil {
			log.Println(convertojson, err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(convertojson)

		return

	}

	_, err := http.Get(currenturl)

	if err != nil {
		mutex.Lock()
		currenturl = backupurl
		mutex.Unlock()
		go WithGo()

	}

	client := &http.Client{}

	resp, err2 := client.Get(currenturl)

	if err2 != nil {
		convertojson, err := json.Marshal(cachedweather)

		if err != nil {
			log.Println(convertojson, err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(convertojson)

		return

		panic(err2)
	}

	defer resp.Body.Close()

	body, dataReadErr := ioutil.ReadAll(resp.Body)
	if dataReadErr != nil {
		panic(dataReadErr)
	}

	var res interface{}
	result := Weatherresult{}

	er1 := json.Unmarshal([]byte(body), &res)
	if er1 != nil {
		log.Println("Marshalling Error", er1)
	}

	if currenturl == primaryurl {

		result = Fromweatherstack(res)

		mutexforcache.Lock()

		clearedcache = result

		mutexforcache.Unlock()

		log.Println("WeatherStack")

	} else {

		result = FromOpenweather(res)
		mutexforcache.Lock()
		clearedcache = result
		mutexforcache.Unlock()
		log.Println("Open weather")
	}

	go ClearingCache()

	convertojson, err := json.Marshal(result)

	if err != nil {
		log.Println(convertojson, err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(convertojson)

	return

}

// reads the data from Weatherstack
func Fromweatherstack(res interface{}) Weatherresult {

	result := Weatherresult{}

	m := res.(map[string]interface{})
	m = m["current"].(map[string]interface{})

	result.Temperature_degrees = int(m["temperature"].(float64))
	result.Wind_speed = int(m["wind_speed"].(float64))

	return result
}

// reads the data from Openweather
func FromOpenweather(res interface{}) Weatherresult {

	result := Weatherresult{}

	m := res.(map[string]interface{})
	m = m["main"].(map[string]interface{})
	todegrees := (m["temp"].(float64) - 273.15)
	result.Temperature_degrees = int(math.Round(todegrees))

	m = res.(map[string]interface{})["wind"].(map[string]interface{})
	result.Wind_speed = int(math.Round(m["speed"].(float64) * 3.6))

	return result
}
