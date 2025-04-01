package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"html/template"
	"io"
	"log"
	"mime/multipart"
	"net/http"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"github.com/joho/godotenv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

var tmpl *template.Template

func main() {
    err := godotenv.Load()
    if err != nil {
	log.Fatal("Error loading .env file")
    }

    tmpl, _ = template.ParseGlob("templates/*.html")

    r := chi.NewRouter()
    r.Use(middleware.Logger)

    r.HandleFunc("/", serveIndex)
    r.HandleFunc("/clicked", buttonClick)
    r.HandleFunc("/chat", chatHandler)
    //r.HandleFunc("/upload", uploadFile)
    r.HandleFunc("/static/*", serveStaticFiles)
    //http.HandleFunc("/ws", nil)

    if err := http.ListenAndServe(":3000", r); err != nil {
        log.Fatal(err)
    }
}

func serveStaticFiles(w http.ResponseWriter, r *http.Request) {
    log.Println("Serving static files")
    // Get file path
    filePath := strings.TrimPrefix(r.URL.Path, "/static/")
    fullPath := filepath.Join("static", filePath)
    log.Printf("Serving: %v\n", fullPath)

    // Set correct content type
    w.Header().Set("Content-Type", "text/css")

    http.ServeFile(w, r, fullPath)
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
    tmpl.ExecuteTemplate(w, "index.html", nil)

}

func buttonClick(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("<h1 id=\"parent-div\">You Clicked Me</h1>"))
}

func mdToHTML(md []byte) []byte {
    // create markdown parser with extensions
    extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.NoEmptyLineBeforeBlock
    p := parser.NewWithExtensions(extensions)
    doc := p.Parse(md)

    // create HTML renderer with extensions
    htmlFlags := html.CommonFlags | html.HrefTargetBlank
    opts := html.RendererOptions{Flags: htmlFlags}
    renderer := html.NewRenderer(opts)

    return markdown.Render(doc, renderer)
}

func chatHandler(w http.ResponseWriter, r *http.Request) {
    err := r.ParseMultipartForm(100 << 20)
    if err != nil {
	log.Println("File too large")
	http.Error(w, "File too large", http.StatusBadRequest)
	return
    }

    message := r.FormValue("message")

    file, handler, err := r.FormFile("png")
    /*if err != nil {
	log.Println("Invalid file")
	http.Error(w, "Invalid file", http.StatusBadRequest)
    }*/
    var uResp *UploadResp
    var agentResponse string
    if file != nil {
	defer file.Close()

	uResp, err = uploadLangflowFile(w, file, handler)
	if err != nil {
	    http.Error(w, "Failed to upload file", http.StatusInternalServerError)
	}
	agentResponse, err = sendChatToLangflow(message, uResp.FilePath)
    } else {
	agentResponse, err = sendChatToLangflow(message, "")
    }

    //_, err = sendChatToLangflow(message, uResp.FilePath)

    mdResponse := template.HTML(mdToHTML([]byte(agentResponse)))

    data := map[string]any{
	"message": message,
	"agentResponse" : mdResponse,
    }
    err = tmpl.ExecuteTemplate(w, "chatResponse.html", data)
    if err != nil {
	log.Printf("Error with template: %v\n", err)
	return
    }
}

func uploadLangflowFile(w http.ResponseWriter, file multipart.File, handler *multipart.FileHeader) (*UploadResp, error) {
    var buf bytes.Buffer
    writer := multipart.NewWriter(&buf)

    part, err := writer.CreateFormFile("file", handler.Filename)
    if err != nil {
	log.Println("Failed to create form file")
	return nil, err
    }
    io.Copy(part, file)
    writer.Close()

    flowID := os.Getenv("FLOW_ID")

    log.Println("Sending file to Langflow")
    uploadURL := "http://localhost:7860/api/v1/files/upload/" + flowID + "?stream=false"
    req, err := http.NewRequest("POST", uploadURL, &buf)
    if err != nil {
	return nil, err
    }

    req.Header.Set("Content-Type", writer.FormDataContentType())

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
	return nil, err
    }
    defer resp.Body.Close()

    responseBody, _ := io.ReadAll(resp.Body)

    log.Printf("Upload repsonse body: %v\n", string(responseBody))

    uploadResp := &UploadResp{}
    err = json.Unmarshal(responseBody, uploadResp)
    if err != nil {
	log.Printf("Langflow error: %v\n", err)
	return nil, err
    }

    return uploadResp, nil
}

type UploadResp struct{
    FlowId string `json:"flowId"`
    FilePath string `json:"file_path"`
}

func sendChatToLangflow(message string, filePath string) (string, error) {

    chatInputID := os.Getenv("CHAT_INPUT_ID")

    componentId := chatInputID
    requestBody := map[string]any{
	//"session_id": "chat-123",
	"input_value": message,
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

    flowID := os.Getenv("FLOW_ID")
    apiURL := "http://localhost:7860/api/v1/run/" + flowID + "?stream=false"

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

    chatResponse := &LangflowChatResponse{}
    json.Unmarshal(responseBody, chatResponse)

    return chatResponse.Outputs[0].Outputs[0].Results.Message.Text, nil
}
