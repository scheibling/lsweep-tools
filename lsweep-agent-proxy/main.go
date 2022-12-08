package main

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/common-nighthawk/go-figure"
	"github.com/joho/godotenv"
)

type Config struct {
	Debug              bool
	Listen             string
	ListenPort         int
	PublicPort         int
	ListenHostname     string
	LSServerHost       string
	LSServerPort       int
	LSServerCert       string
	LSServerIgnoreCert bool
}

// Define global variables
var config Config

func debugLog(message ...any) {
	if config.Debug {
		log.Println(message...)
	}
}

func parseIncomingMultipart(r *http.Request, w http.ResponseWriter) (*multipart.Form, []byte, error) {
	// Parse body data
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Println("Error reading initial incoming request body")
		debugLog(err.Error())
	}

	// Reset original body in request object
	r.Body = ioutil.NopCloser(bytes.NewBuffer(body))

	// Check for faux content-type header (see Step 3 in the protocol overview)
	// and manually set content type and boundary if found
	if r.Header.Get("Content-Type") == "application/x-www-form-urlencoded" {
		debugLog("Content-Type is application/x-www-form-urlencoded")
		debugLog("Overriding this to multipart/form-data and extracting boundary from body")

		mp_separator := strings.Split(string(body), "\r\n")[0][2:]
		r.Header.Set("Content-Type", "multipart/form-data; boundary="+mp_separator)
	}

	if strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
		r.Body = http.MaxBytesReader(w, r.Body, 32<<20+1024)
		reader, err := r.MultipartReader()

		if err != nil {
			fmt.Println("Error parsing multipart request body")
			debugLog(err)
		}

		formdata, err := reader.ReadForm(32 << 20)

		return formdata, body, err
	}
	fmt.Println(r.Header)
	fmt.Println(r.Header.Get("Content-Type"))
	panic("Content-Type is not multipart/form-data")

}

func parseRequestedMultipart(r *http.Response) (*multipart.Form, []byte, error) {
	// Parse body data
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Println("Error reading initial incoming request body")
		debugLog(err.Error())
	}

	// Reset original body in request object
	r.Body = ioutil.NopCloser(bytes.NewBuffer(body))

	mPart := multipart.NewReader(r.Body, r.Header.Get("Content-Type"))
	formdata, err := mPart.ReadForm(32 << 20)

	return formdata, body, err
}

func ctStatus(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	w.Write([]byte("OK"))
}

func lsAgentProxy(w http.ResponseWriter, req *http.Request) {
	debugLog("Request received, parsing multipart data...")

	originalCtype := req.Header.Get("Content-Type")
	formdata, rawBody, err := parseIncomingMultipart(req, w)

	if err != nil {
		fmt.Println("Error parsing multipart request body")
		debugLog(err)
	}

	// Determine the action type
	action := formdata.Value["Action"][0]
	debugLog("Incoming request has action type ", action)

	// Create a new request to the LANSweeper API
	lsreq, err := http.NewRequest(
		"POST",
		"https://"+config.LSServerHost+":"+strconv.Itoa(config.LSServerPort)+"/lsagent",
		bytes.NewReader(rawBody),
	)

	lsreq.Header.Add("Content-Type", originalCtype)
	client := &http.Client{}
	resp, err := client.Do(lsreq)
	if err != nil {
		fmt.Println("Error forwarding request to LANSweeper API")
		debugLog(err)
	}
	defer resp.Body.Close()
	responseFormData, rawResponse, err := parseRequestedMultipart(resp)
	if err != nil {
		debugLog("Response from LANSweeper is not mutlipart/form-data")
		if err.Error() != "multipart: boundary is empty" {
			fmt.Println("Error reading response from LANSweeper API")
			debugLog(err)
		}
	}

	dataChanged := false

	// Check if the original request was for LSAgent Configuration
	if action == "Config" {
		debugLog("Request was for LSAgent Configuration, replacing destination hostname in resposne")

		// Find the server address section in the response
		rg, err := regexp.Compile(`(<Key>Url<\/Key><Value>){1}(http|HTTP){1}(s|S)?:\/\/(.*)(:)?[0-9]+(<\/Value>)+`)
		if err != nil {
			fmt.Println("Error compiling regex")
			debugLog(err)
		}

		isPresent := rg.FindAll(rawResponse, -1)
		if len(isPresent) == 0 {
			fmt.Println("Could not find server address in response, sending unmodified...")
			debugLog("Could not find section matching regex for Lansweeper Server URL Configuration")
		} else {
			debugLog("Replacing", string(isPresent[0]), "with", "https://"+config.ListenHostname+":"+strconv.Itoa(config.PublicPort))
			rawResponse = rg.ReplaceAll(rawResponse, []byte("<Key>Url</Key><Value>https://"+config.ListenHostname+":"+strconv.Itoa(config.PublicPort)+"</Value>"))
			dataChanged = true
		}
	}

	// Write the response from the LANSweeper API to the client
	w.WriteHeader(resp.StatusCode)
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.Write(rawResponse)

	// Echo the request information
	fmt.Println(req.Method + " " + req.URL.Path + " " + req.Proto + " successfully handled")
	debugLog("Request Information:")
	debugLog("  Headers:")
	for k, v := range req.Header {
		debugLog("    ", k, ":", v[0])
	}

	debugLog("  Form data:")
	for k, v := range formdata.Value {
		debugLog("    ", k, ":", v[0])
	}
	debugLog("  Files:")
	for k, v := range formdata.File {
		debugLog("    ", k, ":", v[0])
	}
	debugLog("LANSweeper Response:")
	debugLog("  Headers:")
	for k, v := range resp.Header {
		debugLog("    ", k, ":", v[0])
	}
	if action == "Hello" || action == "ScanData" {
		debugLog("  Body data:")
		debugLog("    ", string(rawResponse))
	} else {
		debugLog("  Form data:")
		for k, v := range responseFormData.Value {
			debugLog("    ", k, ":", v[0])
		}
		debugLog("  Files:")

		for k, v := range responseFormData.File {
			rf, _ := v[0].Open()
			content, _ := ioutil.ReadAll(rf)
			strContent := string(content)

			debugLog("    ", k, ":", strContent[0:20]+"..."+strContent[len(strContent)-150:])
		}
		if dataChanged {
			debugLog("Data was changed in the response back to the Lansweeper Agent")
		}
	}
}

func tryGetEnv(key string, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaultValue
}

func setConfiguration() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Couldn't load .env file: Not present")
		fmt.Println("Using existing environment variables")
	}
	listenPort, err := strconv.Atoi(tryGetEnv("LISTEN_PORT", "8011"))
	if err != nil {
		log.Println("Error parsing LISTEN_PORT, using default value 8011")
		listenPort = 8011
	}

	publicPort, err := strconv.Atoi(tryGetEnv("PUBLIC_PORT", tryGetEnv("LISTEN_PORT", "8011")))
	if err != nil {
		log.Println("Error parsing PUBLIC_PORT, using default value 8011")
		publicPort = listenPort
	}

	lsagentPort, err := strconv.Atoi(tryGetEnv("LSSERVER_PORT", "9524"))
	if err != nil {
		log.Println("Error parsing LSSERVER_PORT, using default value 9524")
		lsagentPort = 9524
	}

	config = Config{
		Debug:              tryGetEnv("DEBUG", "false") == "true",
		Listen:             tryGetEnv("LISTEN", ""),
		ListenPort:         listenPort,
		PublicPort:         publicPort,
		ListenHostname:     tryGetEnv("LISTEN_HOSTNAME", "lsagentproxy.example.com"),
		LSServerHost:       tryGetEnv("LSSERVER_HOST", "lansweeper.example.com"),
		LSServerPort:       lsagentPort,
		LSServerCert:       tryGetEnv("LSSERVER_CERT", ""),
		LSServerIgnoreCert: tryGetEnv("LSSERVER_IGNORE_CERT", "false") == "true",
	}

	if config.Debug {
		fmt.Println("Configuration:")
		fmt.Println("  Debug:", config.Debug)
		fmt.Println("  Listen:", config.Listen)
		fmt.Println("  ListenPort:", config.ListenPort)
		fmt.Println("  PublicPort:", config.PublicPort)
		fmt.Println("  ListenHostname:", config.ListenHostname)
		fmt.Println("  LSServerHost:", config.LSServerHost)
		fmt.Println("  LSServerPort:", config.LSServerPort)
		fmt.Println("  LSServerCert:", config.LSServerCert)
		fmt.Println("  LSServerIgnoreCert:", config.LSServerIgnoreCert)
	} else {
		fmt.Println("Configuration loaded, debugging not enabled. Use DEBUG=true to enable more extensive logging")
	}
}

func main() {
	figure.NewFigure("LSweep", "", true).Print()
	figure.NewFigure("Agent", "", true).Print()
	figure.NewFigure("Proxy", "", true).Print()
	setConfiguration()

	if config.LSServerIgnoreCert {
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	http.HandleFunc("/lsagent", lsAgentProxy)
	http.HandleFunc("/ctstatus", ctStatus)
	http.ListenAndServe(config.Listen+":"+strconv.Itoa(config.ListenPort), nil)
}
