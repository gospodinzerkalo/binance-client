package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/gospodinzerkalo/binance-client/errors"
	"github.com/joho/godotenv"
	"github.com/urfave/cli/v2"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	configPath 				= ".env"
	binanceApiUrl 			= ""
	binanceApiWs 			= ""

	flags = []cli.Flag{
		&cli.StringFlag{
			Name:        "config",
			Usage:       "path to .env config file",
			Aliases: []string{"c"},
			Destination: &configPath,
		},&cli.StringFlag{
			Name:        "symbol",
			Usage:       "symbol, e.g. 'LTCBTC'",
			Aliases: []string{"s"},
		},&cli.StringFlag{
			Name:        "limit",
			Usage:       "Default 100; max 5000. Valid limits:[5, 10, 20, 50, 100, 500, 1000, 5000]",
			Aliases: []string{"l"},
		},
	}
)
func main() {
	app := cli.NewApp()
	app.Commands = cli.Commands{
		&cli.Command{
			Name: "rest",
			Action: rest,
			Flags: flags,
			Usage: "start http rest",
		},
		&cli.Command{
			Name: "ws",
			Action: ws,
			Flags: flags,
			Usage: "start ws",
		},
	}
	parseEnv()
	fmt.Println(app.Run(os.Args))
}

// validation
var limits = map[string]string{
	"5":"5",
	"10":"10",
	"20":"20",
	"50":"50",
	"100":"100",
	"500":"500",
	"1000":"1000",
	"5000":"5000",
}

func parseEnv() {
	if configPath != "" {
		godotenv.Overload(configPath)
	}

	binanceApiUrl = os.Getenv("BINANCE_API_URL")
	binanceApiWs = os.Getenv("BINANCE_API_WS")
}


type Request struct {
	URL 	string
	Method 	string
	Body 	[]byte
}

type Book struct {
	LastUpdateId  		int64		`json:"lastUpdateId"`
	Bids				[][]string	`json:"bids"`
	Asks 				[][]string	`json:"asks"`
}

type Result struct {
	Bids 		[]Item		`json:"bids"`
	Asks 		[]Item		`json:"asks"`
}

type Item struct {
	Price 		string 		`json:"price"`
	Amount 		string 		`json:"amount"`
}

func rest(c *cli.Context) error {
	limit := fmt.Sprintf("%s", c.Value("limit"))
	if _, ok := limits[limit]; len(limits) > 0 && !ok {
		return errors.ErrorLimit
	} else if len(limit) == 0 {
		limit = "10"
	}

	symbol := fmt.Sprintf("%s", c.Value("symbol"))
	if len(symbol) == 0 {
		return errors.ErrorSymbol
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := http.Client{Transport: tr}

	ticker := time.NewTicker(5 * time.Second)

	for {
		select {
		case <-ticker.C:
			resp, err := Get(limit, symbol, client)

			if err != nil {
				log.Fatal(err)
				return err
			}
			log.Println(fmt.Sprintf("%+v", resp))
		}
	}

	return nil
}

func Get(limit, symbol string, cl http.Client) (*Result, error) {
	resp, err := doRequest(Request{
		URL:    fmt.Sprintf("%s/depth?symbol=%s&limit=%s", binanceApiUrl, symbol, limit),
		Method: http.MethodGet,
		Body:   nil,
	}, cl)
	if err != nil {
		return nil, err
	}

	var book Book


	if err := json.Unmarshal(resp, &book); err != nil {
		return nil, err
	}

	result := Result{
		Bids: make([]Item,0),
		Asks: make([]Item, 0),
	}
	for _, v := range book.Bids {
		result.Bids = append(result.Bids, Item{
			Price:  v[0],
			Amount: v[1],
		})
	}
	for _, v := range book.Asks {
		result.Asks = append(result.Asks, Item{
			Price:  v[0],
			Amount: v[1],
		})
	}

	return &result, nil
}

func doRequest(request Request, cl http.Client) ([]byte, error){
	req, err := http.NewRequest(request.Method, request.URL, bytes.NewBuffer(request.Body))
	if err != nil {
		return nil, err
	}

	resp, err := cl.Do(req)
	if err != nil {
		return nil, err
	}

	body, err :=ioutil.ReadAll(resp.Body)

	return body, err
}


// validation
var limitsWs = map[string]string{
	"5":"5",
	"10":"10",
	"20":"20",
}
func ws(c *cli.Context) error {
	limit := fmt.Sprintf("%s", c.Value("limit"))
	if _, ok := limitsWs[limit]; len(limits) > 0 && !ok {
		return errors.ErrorLimitWs
	} else if len(limit) == 0 {
		limit = "10"
	}

	symbol := fmt.Sprintf("%s", c.Value("symbol"))
	if len(symbol) == 0 {
		return errors.ErrorSymbol
	}

	cl, _, err := websocket.DefaultDialer.Dial(fmt.Sprintf("%s/ws/%s@depth%s@100ms", binanceApiWs, strings.ToLower(symbol), limit), nil)

	if err != nil {
		log.Fatal(err)
		return err
	}

	defer cl.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, message, err := cl.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			var book Book
			if err := json.Unmarshal(message, &book); err != nil {
				log.Fatal(err)
				return
			}
			result := Result{
				Bids: make([]Item,0),
				Asks: make([]Item, 0),
			}
			for _, v := range book.Bids {
				result.Bids = append(result.Bids, Item{
					Price:  v[0],
					Amount: v[1],
				})
			}
			for _, v := range book.Asks {
				result.Asks = append(result.Asks, Item{
					Price:  v[0],
					Amount: v[1],
				})
			}
			log.Printf("%+v", result)
		}
	}()
	interrupt := make(chan os.Signal, 1)
	for {
		select {
		case <-done:
			return nil
		case <-interrupt:
			log.Println("interrupt")
			err := cl.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return nil
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return nil
		}
	}
	return nil
}
