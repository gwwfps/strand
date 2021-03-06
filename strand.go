package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"text/template"

	"github.com/kelseyhightower/envconfig"
)

type RequestHandler struct {
	Address             string
	CommandToken        string `envconfig:"command_token"`
	ApiToken            string `envconfig:"api_token"`
	BodyTemplateString  string `envconfig:"template"`
	BodyTemplate        *template.Template
	TitleTemplateString string `envconfig:"title"`
	TitleTemplate       *template.Template
}

type Thread struct {
	ChannelName string
	ChannelId   string
	UserName    string
	Title       string
}

func main() {
	handler := initSettings()

	http.HandleFunc("/alive", handleAlive)
	http.Handle("/command", handler)

	log.Printf("Listening on %s\n", handler.Address)

	log.Fatalln(http.ListenAndServe(handler.Address, nil))
}

func initSettings() *RequestHandler {
	var handler RequestHandler

	err := envconfig.Process("strand", &handler)
	if err != nil {
		log.Fatal("Cannot parse settings:", err)
	}

	if handler.Address == "" {
		handler.Address = "0.0.0.0:8080"
	}

	if handler.CommandToken == "" || handler.ApiToken == "" {
		log.Fatalln("Command token and API token must be provided through environment variables: STRAND_COMMAND_TOKEN and STRAND_API_TOKEN")
	}

	if handler.BodyTemplateString == "" {
		handler.BodyTemplateString = "# {{.Title}}\n\n_This fake thread is auto-generated on behalf of **{{.UserName}}**._"
	}

	if handler.TitleTemplateString == "" {
		handler.TitleTemplateString = "{{.Title}}"
	}

	handler.BodyTemplate = compileTemplate("bodyTemplate", handler.BodyTemplateString)
	handler.TitleTemplate = compileTemplate("titleTemplate", handler.TitleTemplateString)

	return &handler
}

func compileTemplate(name string, templateString string) *template.Template {
	template, err := template.New("titleTemplate").Parse(templateString)
	if err != nil {
		log.Fatal("Cannot parse template:", name, err)
	}
	return template
}

func (handler *RequestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println("Incoming request:", r)

	incomingToken := r.PostFormValue("token")
	if incomingToken != handler.CommandToken {
		log.Println(incomingToken, handler.CommandToken)
		log.Println("Non-matching token in message, ignored.")
		w.WriteHeader(401)
		return
	}

	threadTitle := r.PostFormValue("text")

	if threadTitle == "" {
		log.Println("Empty request, ignoring.")
		fmt.Fprintf(w, "Please provide a topic.")
		return
	}

	thread := Thread{
		Title:       threadTitle,
		ChannelName: r.PostFormValue("channel_name"),
		ChannelId:   r.PostFormValue("channel_id"),
		UserName:    r.PostFormValue("user_name"),
	}

	err := handler.uploadFile(&thread)
	if err != nil {
		fmt.Fprintf(w, "Thread upload failed: %s", err.Error())
	} else {
		fmt.Fprintf(w, "File uploaded to \"%s\" for topic \"%s\".\n", thread.ChannelName, thread.Title)
	}
}

func (handler *RequestHandler) uploadFile(thread *Thread) error {
	formData := url.Values{}
	formData.Set("token", handler.ApiToken)
	formData.Set("filetype", "post")
	formData.Set("channels", thread.ChannelId)

	var bodyContent, titleContent bytes.Buffer
	err := handler.BodyTemplate.Execute(&bodyContent, thread)
	if err == nil {
		err = handler.TitleTemplate.Execute(&titleContent, thread)
	}

	if err != nil {
		log.Println("Cannot parse template:", err)
		return errors.New("There's a problem constructing the file to upload.")
	}

	formData.Set("content", bodyContent.String())
	formData.Set("title", titleContent.String())

	resp, err := http.PostForm("https://slack.com/api/files.upload", formData)
	if err != nil {
		log.Println("Error uploading file:", err, resp)
		return errors.New(fmt.Sprintf("Upload request failed with status %s", resp.Status))
	} else {
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Println("Cannot read upload response:", err)
		}
		log.Println("File uploaded:", string(body))
	}

	return nil
}

func handleAlive(w http.ResponseWriter, req *http.Request) {
	io.WriteString(w, "Listening for requests.\n")
}
