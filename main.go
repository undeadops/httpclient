package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
)

var (
	//proxyUrl = "http://127.0.0.1:8888"
	proxyUrl = "http://proxify:8888"
	brewsUrl = "https://api.openbrewerydb.org/breweries"
	mtlsUrl  = "https://trashcan.undeadops.xyz/hello"

	getCA       bool
	useProxy    bool
	getBrews    bool
	getTrashCan bool
	mtlsFalse   bool

	mtlsClientCert string
	mtlsClientKey  string
)

type Brewery struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	BreweryType    string      `json:"brewery_type"`
	Street         string      `json:"street"`
	Address2       interface{} `json:"address_2"`
	Address3       interface{} `json:"address_3"`
	City           string      `json:"city"`
	State          string      `json:"state"`
	CountyProvince interface{} `json:"county_province"`
	PostalCode     string      `json:"postal_code"`
	Country        string      `json:"country"`
	Longitude      string      `json:"longitude"`
	Latitude       string      `json:"latitude"`
	Phone          string      `json:"phone"`
	WebsiteURL     string      `json:"website_url"`
	UpdatedAt      time.Time   `json:"updated_at"`
	CreatedAt      time.Time   `json:"created_at"`
}

func setupHTTPClient() *http.Client {
	proxyURL, _ := url.Parse(proxyUrl)

	caCert, err := os.ReadFile("rootCA.crt")
	if err != nil {
		log.Fatal(err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool,
			},
			Proxy: http.ProxyURL(proxyURL),
		},
	}

	return client
}

func setupHTTPTransport() *http.Client {
	proxyURL, _ := url.Parse(proxyUrl)

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
	}
	return client
}

func getMyIP(c *http.Client) {
	resp, err := c.Get("http://ifconfig.me/ip")
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}
		bodyString := string(bodyBytes)
		fmt.Printf("%s\n", bodyString)
	}
}

func GetBrews(city string) ([]Brewery, error) {
	// Error checking of city should happen, but meh
	brewurl := fmt.Sprintf("%s?by_city=%s&per_page=5", brewsUrl, city)
	fmt.Println(brewurl)

	client := setupHTTPClient()

	getMyIP(client)

	response, err := client.Get(brewurl)
	if err != nil {
		return []Brewery{}, err
	}

	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return []Brewery{}, err
	}

	responseObject := []Brewery{}
	json.Unmarshal(responseData, &responseObject)

	return responseObject, nil
}

func main() {
	flag.BoolVar(&useProxy, "useproxy", false, "Use Proxify Proxy")
	flag.BoolVar(&getCA, "getca", false, "Download CA Certificate")
	flag.BoolVar(&getBrews, "getbrews", false, "Use proxy to featch list of breweries from api")
	flag.BoolVar(&getTrashCan, "gettrash", false, "Fetch local HTTP endpoint using client certs")
	flag.BoolVar(&mtlsFalse, "nocerts", false, "Do Not supply TLS Credentials")
	flag.StringVar(&mtlsClientCert, "cert", "client.pem", "mTLS Client Certificate")
	flag.StringVar(&mtlsClientKey, "key", "client-key.pem", "mTLS Client Certificate Key")
	flag.Parse()

	if getCA {
		client := setupHTTPTransport()

		resp, err := client.Get("http://proxify/cacert.crt")
		if err != nil {
			fmt.Printf("Error fetching caCert pem, %v", err.Error())
		}
		defer resp.Body.Close()

		// Create the file
		out, err := os.Create("rootCA.crt")
		if err != nil {
			fmt.Printf("error writing rootca.crt - %v\n", err.Error())
		}
		defer out.Close()

		// Write the body to file
		_, err = io.Copy(out, resp.Body)
	}

	if getBrews {
		b, err := GetBrews("san_diego")
		if err != nil {
			fmt.Printf("Error Getting San Diego, %s", err)
		}
		fmt.Println(fmt.Sprintf("%v", b))
	}

	if getTrashCan {
		cert, err := tls.LoadX509KeyPair(mtlsClientCert, mtlsClientKey)
		if err != nil {
			log.Fatal(err)
		}

		proxy := &url.URL{}
		client := &http.Client{}
		if useProxy {
			fmt.Printf("Connecting to Trashcan using proxy: %s\n", proxyUrl)
			proxy, _ = url.Parse(proxyUrl)
			caCert, err := os.ReadFile("rootCA.crt")
			if err != nil {
				log.Fatal(err)
			}
			caCertPool := x509.NewCertPool()
			caCertPool.AppendCertsFromPEM(caCert)

			client = &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						RootCAs:            caCertPool,
						Certificates:       []tls.Certificate{cert},
						InsecureSkipVerify: true, // Because trashcan is selfi signed
					},
					Proxy: http.ProxyURL(proxy),
				},
			}
		} else {
			fmt.Printf("Connecting to Trashcan without Proxy\n")
			client = &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						Certificates:       []tls.Certificate{cert},
						InsecureSkipVerify: true, // Because trashcan is selfi signed
					},
				},
			}
		}

		req, err := http.NewRequest("GET", mtlsUrl, nil)
		if err != nil {
			log.Fatalln(err)
		}
		req.Header.Set("User-Agent", "httpproxy/1.0")
		r, err := client.Do(req)
		if err != nil {
			log.Fatal(err)
		}

		defer r.Body.Close()
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s\n", body)
	}
}
