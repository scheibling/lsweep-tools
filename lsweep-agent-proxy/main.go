package main

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
)

func lsagentproxy(w http.ResponseWriter, req *http.Request) {
	fmt.Println(req.Method + " " + req.URL.Path + " " + req.Proto)
	body, err := io.ReadAll(req.Body)
	if err != nil {
		fmt.Println("Error reading body: ", err)
	}
	mp_separator := strings.Split(string(body), "\r\n")[0]
	// mp_separator := "----------------------------833124210359601947518624"
	req.Header["Content-Type"][0] = "multipart/form-data; boundary=" + mp_separator[2:]
	mpread, err := multipart.NewReader(strings.NewReader(string(body)), req.Header.Get("Content-Type")).ReadForm(32 << 20)
	if err != nil {
		fmt.Println("Error reading multipart form: ", err)
	}
	for name, value := range req.Header {
		val := strings.Join(value, ",")
		fmt.Println(name + ": " + val)
	}

	fmt.Println(mpread.Value)

	// fmt.Println(req.ParseMultipartForm(32 << 20))

	// fmt.Println(req.MultipartForm)

	// fmt.Println(string(body))
	fmt.Println()
}

func main() {
	http.HandleFunc("/lsagent", lsagentproxy)
	http.ListenAndServe(":8011", nil)
}
