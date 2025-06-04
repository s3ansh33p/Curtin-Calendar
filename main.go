package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"path/filepath"
	"sync"

	ics "github.com/arran4/golang-ical"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/joho/godotenv"
)

type Calendar struct {
	Name    string     `json:"name"`
	Domains [][]string `json:"domains"`
}

type Calendars struct {
	Calendars []Calendar `json:"calendars"`
}

func fetchICal(url string) (*ics.Calendar, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	cal, err := ics.ParseCalendar(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	return cal, nil
}

func uploadToR2(sess *session.Session, bucket, key, filePath string) error {
	svc := s3.New(sess)
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = svc.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   file,
	})
	return err
}

func main() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file")
		return
	}

	accessKey := os.Getenv("ACCESS_KEY")
	secretKey := os.Getenv("SECRET_KEY")
	accountID := os.Getenv("ACCOUNT_ID")
	bucketName := os.Getenv("BUCKET_NAME")

	file, err := os.Open("icals.json")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer file.Close()

	byteValue, err := io.ReadAll(file)
	if err != nil {
		fmt.Println(err)
		return
	}

	var calendars Calendars
	err = json.Unmarshal(byteValue, &calendars)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Initialize AWS session
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("auto"),
		Credentials: credentials.NewStaticCredentials(accessKey, secretKey, ""),
		Endpoint:    aws.String("https://" + accountID + ".r2.cloudflarestorage.com"),
	})
	if err != nil {
		fmt.Println("Failed to create session:", err)
		return
	}

	var wg sync.WaitGroup
	for _, calendar := range calendars.Calendars {
		cal := ics.NewCalendar()
		cal.SetProductId("-//s3ansh33p//Curtin-Clubs//EN")

		for _, domainPair := range calendar.Domains {
			wg.Add(1)
			go func(domain string) {
				defer wg.Done()
				ical, err := fetchICal("https://" + domain + ".tidyhq.com/public/schedule/events.ics")
				if err != nil {
					fmt.Println(err)
					return
				}

				var PRODID string
				for _, prop := range ical.CalendarProperties {
					if prop.IANAToken == string(ics.PropertyProductId) {
						PRODID = prop.Value
					}
				}

				numEvents := len(ical.Events())
				fmt.Println("Fetched " + domain + " (" + PRODID + ") - " + fmt.Sprintf("%d events", numEvents))
				for _, event := range ical.Events() {
					description := PRODID + "\nURL: " + event.GetProperty("URL").Value + "\n" + event.GetProperty("DESCRIPTION").Value
					event.SetDescription(description)
					cal.AddVEvent(event)
				}
			}(domainPair[0])
		}

		wg.Wait()

		err = os.MkdirAll("output", 0755)
		if err != nil {
			fmt.Println(err)
			return
		}
		filePath := "output/" + calendar.Name + ".ics"
		file, err := os.Create(filePath)
		if err != nil {
			fmt.Println(err)
			return
		}
		defer file.Close()
		_, err = file.WriteString(cal.Serialize())
		if err != nil {
			fmt.Println(err)
			return
		}

		err = uploadToR2(sess, bucketName, filepath.Base(filePath), filePath)
		if err != nil {
			fmt.Println("Failed to upload file:", err)
			return
		}
		fmt.Println("Uploaded " + filePath + " to R2")
	}
}
