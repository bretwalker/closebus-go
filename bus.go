package main

import (
    "encoding/csv"
    "encoding/json"
    "fmt"
    "io"
    "io/ioutil"
    "log"
    "math"
    "net/http"
    "os"
    "sort"
    "strconv"
    "time"
)

import (
    "code.google.com/p/goprotobuf/proto"
    
    // This is just a complied version of the output of
    // http://code.google.com/p/goprotobuf/ run on the GTFS-realtime proto file
    "megaminor.com/go/realtime"
)

var busses []bus = []bus{}
var trips map[string]string = map[string]string{}
var routes map[string]string = map[string]string{}
var tripsToRoutes map[string]string = map[string]string{}

type point struct {
    Lat, Lon float32
}

type bus struct {
    TripId, TripDescription, RouteDescription, BusLabel string
    Point point
    Distance float64
}

type By func(b1, b2 *bus) bool

func (by By) Sort(busses []bus) {
	ps := &busSorter{
		busses: busses,
		by:      by, // The Sort method's receiver is the function (closure) that defines the sort order.
	}
	sort.Sort(ps)
}

type busSorter struct {
	busses []bus
	by      func(b1, b2 *bus) bool // Closure used in the Less method.
}

// Len is part of sort.Interface.
func (s *busSorter) Len() int {
	return len(s.busses)
}

// Swap is part of sort.Interface.
func (s *busSorter) Swap(i, j int) {
	s.busses[i], s.busses[j] = s.busses[j], s.busses[i]
}

// Less is part of sort.Interface. It is implemented by calling the "by" closure in the sorter.
func (s *busSorter) Less(i, j int) bool {
	return s.by(&s.busses[i], &s.busses[j])
}

func toRad(f float32) (float64) {
    return float64(f * math.Pi/180)
}

func calculateDistances(p point, myBusses []bus) ([]bus) {
    var r float64 = 6371
    
    lat1 := toRad(p.Lat)
    lon1 := toRad(p.Lon)
    
    for key, value := range myBusses {
        lat2 := toRad(value.Point.Lat)
        lon2 := toRad(value.Point.Lon)
        
        myBusses[key].Distance = math.Acos(math.Sin(lat1) * math.Sin(lat2) + math.Cos(lat1) * math.Cos(lat2) * math.Cos(lon2 - lon1)) * r
    }
    
    return myBusses
}

func loadBusLocations() () {
   f, _ := ioutil.ReadFile("VehiclePositions.pb")

   newMessage := &transit_realtime.FeedMessage{}
   err := proto.Unmarshal(f, newMessage)
   if err != nil {   
       log.Println(err)
   }

   newBusses := []bus{}

   for _, value := range newMessage.Entity {
       newPoint := point{*value.Vehicle.Position.Latitude, *value.Vehicle.Position.Longitude}
       newBus := bus{*value.Vehicle.Trip.TripId, trips[*value.Vehicle.Trip.TripId], routes[tripsToRoutes[*value.Vehicle.Trip.TripId]], *value.Vehicle.Vehicle.Label, newPoint, 0}
       newBusses = append(newBusses, newBus)
   }
   
   busses = newBusses
}

func loadTripsOrRoutes(filePath string, m map[string]string, keyPosition, valuePosition int) () {
    csvFile, err := os.Open(filePath)
    defer csvFile.Close()
    if err != nil {
        log.Println(err)
    }
    csvReader := csv.NewReader(csvFile)
    for {
        fields, err := csvReader.Read()
        if err == io.EOF {
            break
        } else if err != nil {
            log.Println(err)
        }
        m[fields[keyPosition]] = fields[valuePosition]
    }
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprint(w, "Coming soon...")
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
    lat, latOK := strconv.ParseFloat(r.FormValue("lat"), 32)
    lon, lonOK := strconv.ParseFloat(r.FormValue("lon"), 32)
    
    if latOK != nil && lonOK != nil {
        http.Error(w, "403 Forbidden: Latitude and longitude required to view this page.", 403)
        return
    }

    currentLocation := point{float32(lat), float32(lon)}
    myBusses := calculateDistances(currentLocation, busses)

    distance := func(b1, b2 *bus) bool {
    	return b1.Distance < b2.Distance
    }

    By(distance).Sort(myBusses)
    
    allBusses, _ := strconv.ParseInt(r.FormValue("allBusses"), 10, 0)
        
    if allBusses == 0 {
        myBusses = myBusses[:1]
    }
    
    b, _ := json.Marshal(myBusses)
    
    w.Header().Set("Content-Type", "application/json")
    s := string(b)
    fmt.Fprint(w, s)
}

func main() {
    pwd, _ := os.Getwd()
    log.Println(pwd)
    loadTripsOrRoutes("routes.txt", routes, 0, 3)
    loadTripsOrRoutes("trips.txt", trips, 2, 3)
    loadTripsOrRoutes("trips.txt", tripsToRoutes, 2, 0)
    
    go func() {
        for {
            res, err := http.Get("http://www.example.com/VehiclePositions.pb")
        	if err != nil {
        		log.Println(err)
        	}
        	vehiclePositions, err := ioutil.ReadAll(res.Body)
        	res.Body.Close()
        	if err != nil {
        		log.Println(err)
        	}
        	
        	ioutil.WriteFile("VehiclePositions.pb", vehiclePositions, 0444)
        	
        	loadBusLocations()
        	        	
            time.Sleep(70 * time.Second)
        }
    }()

    s := &http.Server{
    	Addr:           ":8080",
    	Handler:        nil,
    	ReadTimeout:    10 * time.Second,
    	WriteTimeout:   10 * time.Second,
    	MaxHeaderBytes: 1 << 20,
    }
    
    http.HandleFunc("/", homeHandler)
    http.HandleFunc("/status", statusHandler)
    err :=  s.ListenAndServe()
    
    if err != nil {
        log.Println(err)
        os.Exit(1)
    }
}
