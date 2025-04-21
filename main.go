package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/util"
	"github.com/evcc-io/evcc/util/request"
	"github.com/evcc-io/evcc/util/transport"
	"github.com/fatih/structs"
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/now"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v3"
)

type price struct {
	Peak    float64
	OffPeak float64 `yaml:"off-peak"`
}

type config struct {
	ClientID     string
	ClientSecret string
	Prices       struct {
		Blue, Red, White price
	}
}

func backoffPermanentError(err error) error {
	if se := new(request.StatusError); errors.As(err, &se) {
		if code := se.StatusCode(); code >= 400 && code <= 599 {
			return backoff.Permanent(se)
		}
	}
	if err != nil && strings.HasPrefix(err.Error(), "jq: query failed") {
		return backoff.Permanent(err)
	}
	return err
}

func bo() backoff.BackOff {
	return backoff.NewExponentialBackOff(
		backoff.WithInitialInterval(time.Second),
		backoff.WithMaxElapsedTime(time.Minute),
	)
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func readConfig() (*config, error) {
	configFile := getEnv("CONFIG_FILE", "/etc/evcc-tempo/evcc-tempo.yaml")
	buf, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	c := &config{}
	err = yaml.Unmarshal(buf, c)
	if err != nil {
		return nil, fmt.Errorf("in file %q: %w", configFile, err)
	}

	return c, err
}

func validTempoValue(lookup string) bool {
	switch lookup {
	case
		"BLUE",
		"WHITE",
		"RED":
		return true
	}
	return false
}

func main() {
	conf, err := readConfig()
	if err != nil {
		log.Panicln(err)
	}
	fmt.Println(fmt.Sprintf("conf, %#v", conf))
	configPrices := structs.Map(conf.Prices)
	if len(configPrices) != 3 {
		log.Panicln(errors.New("missing prices for red/blue/white"))
	}
	prices := make(map[string]price)
	for k, v := range configPrices {
		mapPrices := v.(map[string]interface{})
		price := &price{
			OffPeak: mapPrices["OffPeak"].(float64),
			Peak:    mapPrices["Peak"].(float64),
		}
		prices[strings.ToLower(k)] = *price
	}
	basic := transport.BasicAuthHeader(conf.ClientID, conf.ClientSecret)
	log := util.NewLogger("edf-tempo").Redact(basic)
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
	r.GET("/prices", func(c *gin.Context) {
		rteUrl := "https://digital.iservices.rte-france.com"
		rteTokenUrl := fmt.Sprintf("%s/token/oauth/", rteUrl)
		rteTempoCalendar := fmt.Sprintf("%s/open_api/tempo_like_supply_contract/v1/tempo_like_calendars", rteUrl)
		tokenRequest, _ := request.New(http.MethodPost, rteTokenUrl, nil, map[string]string{
			"Authorization": basic,
			"Content-Type":  request.FormContent,
			"Accept":        request.JSONContent,
		})
		var token oauth2.Token
		client := request.NewHelper(log)
		log.DEBUG.Print("Retrieving token")
		err = client.DoJSON(tokenRequest, &token)
		if err != nil {
			log.ERROR.Print("Error retrieving RTE token", err)
		}
		log.DEBUG.Print("Retrieved token")

		var res struct {
			Data struct {
				Values []struct {
					StartDate time.Time `json:"start_date"`
					EndDate   time.Time `json:"end_date"`
					Value     string    `json:"value"`
				} `json:"values"`
			} `json:"tempo_like_calendars"`
		}

		today := now.BeginningOfDay()
		start := today.AddDate(0, 0, -1)
		end := today.AddDate(0, 0, 2)

		uri := fmt.Sprintf("%s?start_date=%s&end_date=%s&fallback_status=true",
			rteTempoCalendar,
			strings.ReplaceAll(start.Format(time.RFC3339), "+", "%2B"),
			strings.ReplaceAll(end.Format(time.RFC3339), "+", "%2B"))

		tempoRequest, _ := request.New(http.MethodGet, uri, nil, map[string]string{
			"Authorization": "Bearer " + token.AccessToken,
			"Accept":        request.JSONContent,
		})

		if err := backoff.Retry(func() error {
			return backoffPermanentError(client.DoJSON(tempoRequest, &res))
		}, bo()); err != nil {
			log.ERROR.Println(err)
		}

		data := make(api.Rates, 0, 2*len(res.Data.Values))

		for _, r := range res.Data.Values {
			// filter values that we know how to deal with
			if validTempoValue(r.Value) {
				peakPrice := prices[strings.ToLower(r.Value)].Peak
				offPeakPrice := prices[strings.ToLower(r.Value)].OffPeak
				arPeak := api.Rate{
					Start: r.StartDate.Local().Add(time.Hour * 6).UTC(),
					End:   r.StartDate.Local().Add(time.Hour * 22).UTC(),
					Value: peakPrice,
				}
				data = append(data, arPeak)
				arOffPeak := api.Rate{
					Start: r.StartDate.Local().Add(time.Hour * 22).UTC(),
					End:   r.StartDate.Local().Add(time.Hour * 30).UTC(),
					Value: offPeakPrice,
				}
				data = append(data, arOffPeak)
			}
		}
		sort.Slice(data, func(i, j int) bool {
			return data[i].Start.Before(data[j].Start)
		})

		c.JSON(200, data)

	})
	r.Run() // listen and serve on 0.0.0.0:8080
}
