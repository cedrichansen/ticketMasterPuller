package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jamespearly/loggly"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

type DynamoDbEvent struct {
	EventName     string
	ID            string
	VenueName     string
	StartDateTime string
	StartDate     string
	City          string
}

type Location struct {
	IP            string  `json:"ip"`
	Type          string  `json:"type"`
	ContinentCode string  `json:"continent_code"`
	ContinentName string  `json:"continent_name"`
	CountryCode   string  `json:"country_code"`
	CountryName   string  `json:"country_name"`
	RegionCode    string  `json:"region_code"`
	RegionName    string  `json:"region_name"`
	City          string  `json:"city"`
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
}

type Response struct {
	Summary Summary `json:"_embedded"`
}

type Summary struct {
	Events []Event `json:"events"`
}

type Event struct {
	Name         string       `json:"name"`
	ID           string       `json:"id"`
	Date         Date         `json:"dates"`
	PriceRange   []PriceRange `json:"priceRanges"`
	EmbeddedData EmbeddedData `json:"_embedded"`
}

type EmbeddedData struct {
	Venues []Venue `json:"venues"`
}

type Venue struct {
	Name string `json:"name"`
}

type Date struct {
	StartDateTime Start `json:"start"`
	EndDateTime   End   `json:"end"`
}

type PriceRange struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

type Start struct {
	LocalDate string `json:"localDate"`
	LocalTime string `json:"localTime"`
}

type End struct {
	LocalDate string `json:"localDate"`
	LocalTime string `json:"localTime"`
}

/*
	this function sends an http get request, running the function "GetEvents function" using the city name as a paramter.
*/
func GetEventsInCity(city string) {

	key, found := os.LookupEnv("TICKET_MASTER_KEY")
	if !found {
		client := loggly.New("csc484")
		logglyResp := client.EchoSend("error", "Cannot find env variable: TICKET_MASTER_KEY")
		fmt.Println("error:", logglyResp)
		os.Exit(0)
	}

	city = strings.Replace(city, " ", "+", -1)
	currentTime := time.Now()
	//endDate := currentTime.AddDate(0, 0, 14)

	fmt.Println("Available events after " + currentTime.Format("2006-01-02") + " in " + city + "\n")

	url := "https://app.ticketmaster.com/discovery/v2/events.json?apikey=" + key + "&city=" + city + "&sort=date,asc&startDateTime=" + currentTime.Format("2006-01-02") + "T14:00:00Z&startDate=" + currentTime.Format("2006-01-02") + "T14:00:00Z"
	resp, err := http.Get(url)
	checkErr("Cannot complete http GET request", err)

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	checkErr("Cannot read the returned json", err)

	var result Response
	err = json.Unmarshal(body, &result)
	checkErr("Cannot properly unmarshal the ticketmaster data", err)

	client := loggly.New("csc484")
	logglyMsg := eventList(result)
	logglyMsg = "Events in " + city + "\n" + logglyMsg

	sendResponseToDynamoDB(result, city)

	logglyResp := client.EchoSend("info", logglyMsg+"\nAdded items to dynamo db")

	if logglyResp != nil {
		fmt.Print("info:", logglyMsg)
	}

}

func sendResponseToDynamoDB(response Response, city string) {

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1")},
	)

	checkErr("Something went wrong with aws config", err)

	// Create DynamoDB client
	svc := dynamodb.New(sess)

	length := len(response.Summary.Events)

	for i := 0; i < length; i++ {
		event := response.Summary.Events[i]

		e := DynamoDbEvent{
			EventName:     event.Name,
			ID:            event.ID,
			VenueName:     event.EmbeddedData.Venues[0].Name,
			StartDateTime: event.Date.StartDateTime.LocalTime,
			StartDate:     event.Date.StartDateTime.LocalDate,
			City:          city,
		}

		av, err := dynamodbattribute.MarshalMap(e)

		input := &dynamodb.PutItemInput{
			Item:      av,
			TableName: aws.String("Ticketmaster_events"),
		}

		_, err = svc.PutItem(input)

		checkErr("Got error calling PutItem", err)

		fmt.Print("Succesfully addded all items to DynamoDB!")

	}

}

func printEvents(r Response) {
	for i := 0; i < len(r.Summary.Events); i++ {
		info := r.Summary.Events[i].Name + "  " + r.Summary.Events[i].Date.EndDateTime.LocalDate
		fmt.Println(info)
	}
	fmt.Println()
}

func eventList(r Response) string {
	var evnts []string

	for i := 0; i < len(r.Summary.Events); i++ {

		//some events occur over multiple days, and will have both a start and end date. If there is an end date,
		//print the end date, but if it is only a single day event, only print out that event
		if r.Summary.Events[i].Date.EndDateTime.LocalDate == "" {
			info := r.Summary.Events[i].Name + " " + r.Summary.Events[i].Date.StartDateTime.LocalDate + "\n"
			evnts = append(evnts, info)
		} else {
			info := r.Summary.Events[i].Name + " " + r.Summary.Events[i].Date.EndDateTime.LocalDate + "\n"
			evnts = append(evnts, info)
		}
	}

	ret := strings.Join(evnts, "")

	return ret
}

func checkErr(message string, err error) {
	if err != nil {
		client := loggly.New("csc484")
		logglyResp := client.EchoSend("error", message)
		fmt.Println("error:", logglyResp)
		fmt.Println(err)
		os.Exit(1)
	}
}

func main() {
	for true {
		GetEventsInCity("Ottawa")
		time.Sleep(12 * time.Hour)
	}

}
