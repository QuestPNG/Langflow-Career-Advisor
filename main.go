package main

import (
	"bytes"
	"encoding/json"
	"fmt"

	//"fmt"
	"html/template"
	"io"
	"log"
	"mime/multipart"
	"net/http"

	//"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

var tmpl *template.Template

func main() {

    tmpl, _ = template.ParseGlob("templates/*.html")

    r := chi.NewRouter()
    r.Use(middleware.Logger)

    r.HandleFunc("/", serveIndex)
    r.HandleFunc("/clicked", buttonClick)
    r.HandleFunc("/chat", chatResponse)
    r.HandleFunc("/upload", uploadFile)
    //http.HandleFunc("/ws", nil)

    if err := http.ListenAndServe(":3000", r); err != nil {
        log.Fatal(err)
    }
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
    tmpl.ExecuteTemplate(w, "index.html", nil)

}

func buttonClick(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("<h1 id=\"parent-div\">You Clicked Me</h1>"))
}

func chatResponse(w http.ResponseWriter, r *http.Request) {
    //msg, _ := io.ReadAll(r.Body)
    
    //log.Printf("Message: %v\n", string(msg))

    message := r.FormValue("message")

    log.Printf("Message: %v\n", message)

    data := map[string]string{"message": message}
    tmpl.ExecuteTemplate(w, "chatResponse.html", data)
}

type UploadResp struct{
    FlowId string `json:"flowId"`
    FilePath string `json:"file_path"`
}

func uploadFile(w http.ResponseWriter, r *http.Request) {
    err := r.ParseMultipartForm(100 << 20)
    if err != nil {
	log.Println("File too large")
	http.Error(w, "File too large", http.StatusBadRequest)
	return
    }

    file, header, err := r.FormFile("png")
    if err != nil {
	log.Println("Invalid file")
	http.Error(w, "Invalid file", http.StatusBadRequest)
    }
    defer file.Close()

    var buf bytes.Buffer
    writer := multipart.NewWriter(&buf)

    part, err := writer.CreateFormFile("file", header.Filename)
    if err != nil {
	http.Error(w, "Failed to create form file", http.StatusInternalServerError)
	return
    }
    io.Copy(part, file)
    writer.Close()

    /*dst, err := os.Create("./uploads/" + header.Filename)
    if err != nil {
	log.Println("Failed to save file")
	http.Error(w, "Failed to save file", http.StatusInternalServerError)
    }
    defer dst.Close()
    io.Copy(dst, file)*/

    //Forward file to Langflow API

    log.Println("Sending file to Langflow")
    uploadURL := "http://localhost:7860/api/v1/files/upload/37377164-c4e0-40c5-9a7f-872f3931349d?stream=false"
    req, err := http.NewRequest("POST", uploadURL, &buf)
    if err != nil {
	http.Error(w, "Failed to create request", http.StatusInternalServerError)
	return
    }

    req.Header.Set("Content-Type", writer.FormDataContentType())

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
	http.Error(w, "Failed to send file to Langflow API", http.StatusInternalServerError)
    }
    defer resp.Body.Close()

    responseBody, _ := io.ReadAll(resp.Body)

    log.Printf("Upload response body: %v", string(responseBody))
    
    uploadResp := &UploadResp{}

    err = json.Unmarshal(responseBody, uploadResp)
    if err != nil {
	http.Error(w, "Error while uploading file to Langflow API", http.StatusInternalServerError)
	log.Printf("Langflow error: %v", err)
	return
    }

    langflowResponse, err := sendToLangflow(uploadResp.FilePath)
    if err != nil {
	http.Error(w, "Failed to send file to Langflow", http.StatusInternalServerError)
	return
    }

    finalResp := fmt.Sprintf("<p>Langflow response: %v</p>", langflowResponse)

    w.Write([]byte(finalResp))
    //w.Write(responseBody)


    /*
    resp := fmt.Sprintf("<p>File uploaded successfully: %v</p>", header.Filename)
    w.Write([]byte(resp))
    */
}

func sendToLangflow(filePath string) (string, error) {

    componentId := "ChatInput-XNOtH"
    requestBody := map[string]any{
	"output_type": "chat",
	"input_type": "chat",
	"tweaks": map[string]any{
	    componentId: map[string]string{
		"files": filePath,
	    },
	},
    }

    jsonData, err := json.Marshal(requestBody)
    if err != nil {
	return "", err
    }

    apiURL := "http://localhost:7860/api/v1/run/37377164-c4e0-40c5-9a7f-872f3931349d?stream=false"

    req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
    if err != nil {
	return "", err
    }

    req.Header.Set("Content-Type", "application/json")

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
	return "", err
    }
    defer resp.Body.Close()

    responseBody, err := io.ReadAll(resp.Body)
    if err != nil {
	return "", err
    }

    return string(responseBody), nil
}
