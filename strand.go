package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
)

type requestHandler struct {
	commandToken string
	apiToken     string
}

func main() {
	if len(os.Args) < 3 {
		log.Fatalln("Usage: strandserver [-address=<address>] <slack command token> <upload API token>")
	}

	address := flag.String("address", "0.0.0.0:8080", "address to listen on")
	flag.Parse()
	handler := requestHandler{commandToken: flag.Arg(0), apiToken: flag.Arg(1)}

	http.Handle("/", &handler)

	log.Printf("Listening on %s\n", *address)

	log.Fatalln(http.ListenAndServe(*address, nil))
}

func (handler *requestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println("Incoming request:", r)

	incomingToken := r.PostFormValue("token")
	if incomingToken != handler.commandToken {
		log.Println(incomingToken, handler.commandToken)
		log.Println("Non-matching token in message, ignored.")
		w.WriteHeader(401)
		return
	}

	threadTitle := r.PostFormValue("text")
	channelName := r.PostFormValue("channel_name")
	channelId := r.PostFormValue("channel_id")
	userName := r.PostFormValue("user_name")

	go uploadFile(handler.apiToken, threadTitle, userName, channelId)

	fmt.Fprintf(w, "File upload to \"%s\" for topic \"%s\" on its way.", channelName, threadTitle)
}

func uploadFile(apiToken string, threadTitle string, userName string, channelId string) {
	formData := url.Values{}
	formData.Set("token", apiToken)
	formData.Set("content", fmt.Sprintf("Thread topic: %s\nThis fake thread is auto-generated on behalf of @%s.", threadTitle, userName))
	formData.Set("channels", channelId)
	formData.Set("title", fmt.Sprintf("%s's thread: %s", userName, threadTitle))

	resp, err := http.PostForm("https://slack.com/api/files.upload", formData)
	if err != nil {
		log.Println("Error uploading file:", err)
	} else {
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Println("Cannot read upload response:", err)
		}
		log.Println("File uploaded:", string(body))
	}
}
