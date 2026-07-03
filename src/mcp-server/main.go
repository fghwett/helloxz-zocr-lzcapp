package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Config holds the configuration options.
type Config struct {
	ApiURL  string
	Token   string
	Port    string
	BaseURL string
}

// ZOCRResponse represents the response format of helloxz/zocr API.
type ZOCRResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Texts    []string  `json:"texts"`
		Scores   []float64 `json:"scores"`
		Boxes    [][][]int `json:"boxes"`
		FullText string    `json:"full_text"`
	} `json:"data"`
}

const maxImageBytes int64 = 10 << 20

func main() {
	// 1. Load config
	apiURL := os.Getenv("ZOCR_API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:5080"
	}
	token := os.Getenv("ZOCR_TOKEN")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	baseURL := os.Getenv("MCP_BASE_URL")
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://localhost:%s", port)
	}

	cfg := Config{
		ApiURL:  apiURL,
		Token:   token,
		Port:    port,
		BaseURL: baseURL,
	}

	log.Printf("Starting ZOCR MCP Server...")
	log.Printf("ZOCR API URL: %s", cfg.ApiURL)
	log.Printf("MCP Base URL: %s", cfg.BaseURL)

	// 2. Initialize MCP Server
	s := server.NewMCPServer(
		"ZOCR OCR Service",
		"1.0.0",
		server.WithDescription("MCP server providing OCR capabilities using helloxz/zocr"),
	)

	// 3. Register tools
	// Tool: OCR local image file
	s.AddTool(mcp.NewTool("ocr_image_file",
		mcp.WithDescription("Recognize text from an image (using base64 string or local file path) using ZOCR"),
		mcp.WithString("image_base64",
			mcp.Description("The base64 encoded string of the image file (supports data URI scheme prefix)"),
		),
		mcp.WithString("file_path",
			mcp.Description("The absolute path to the local image file on disk (only works if MCP client and server share the same disk)"),
		),
	), createOCRFileHandler(cfg))

	// Tool: OCR remote image URL
	s.AddTool(mcp.NewTool("ocr_image_url",
		mcp.WithDescription("Recognize text from a remote image URL using ZOCR"),
		mcp.WithString("url",
			mcp.Description("The HTTP(S) URL of the image to perform OCR on"),
			mcp.Required(),
		),
	), createOCRURLHandler(cfg))

	// 4. Initialize Streamable HTTP Server
	streamableServer := server.NewStreamableHTTPServer(
		s,
		server.WithEndpointPath("/mcp"),
	)

	// 5. Setup custom router to mount Streamable HTTP handler
	mux := http.NewServeMux()
	mux.Handle("/mcp", streamableServer)
	mux.Handle("/mcp/", streamableServer) // Handle sub-paths if any
	mux.HandleFunc("/ocr/upload", createOCRUploadHandler(cfg))

	// Add a simple health check or root index
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":       "running",
			"server":       "ZOCR MCP Server",
			"transport":    "Streamable HTTP",
			"mcp_endpoint": "/mcp",
		})
	})

	addr := ":" + cfg.Port
	log.Printf("Listening for MCP clients on %s (Streamable HTTP: %s/mcp)", addr, cfg.BaseURL)
	httpServer := &http.Server{
		Addr:           addr,
		Handler:        mux,
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   60 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	if err := httpServer.ListenAndServe(); err != nil {
		log.Fatalf("Failed to run HTTP server: %v", err)
	}
}

func createOCRUploadHandler(cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxImageBytes+(1<<20))
		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "file form field is required", http.StatusBadRequest)
			return
		}
		defer file.Close()

		zocrResp, err := callZOCRUpload(r.Context(), cfg, header.Filename, file)
		if err != nil {
			status := http.StatusBadGateway
			if strings.Contains(err.Error(), "exceeds max size") {
				status = http.StatusRequestEntityTooLarge
			}
			http.Error(w, err.Error(), status)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(zocrResp)
	}
}

// createOCRFileHandler creates the tool handler for ocr_image_file.
func createOCRFileHandler(cfg Config) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		base64Data := request.GetString("image_base64", "")
		filePath := request.GetString("file_path", "")

		log.Printf("Received ocr_image_file request. image_base64 length: %d chars, file_path: %q", len(base64Data), filePath)

		if base64Data == "" && filePath == "" {
			return mcp.NewToolResultError("either image_base64 or file_path argument must be provided"), nil
		}

		var reader io.Reader
		var filename string

		if base64Data != "" {
			filename = "image." + imageExtensionFromBase64(base64Data)

			decodedBytes, err := decodeBase64Image(base64Data)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to decode base64 image data: %v", err)), nil
			}
			reader = bytes.NewReader(decodedBytes)
		} else {
			// Read local file
			file, err := os.Open(filePath)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to open file %q: %v", filePath, err)), nil
			}
			defer file.Close()
			reader = file
			filename = filepath.Base(filePath)
		}

		zocrResp, err := callZOCRUpload(ctx, cfg, filename, reader)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Return both text representation and JSON structured data
		return mcp.NewToolResultStructured(zocrResp.Data, zocrResp.Data.FullText), nil
	}
}

// createOCRURLHandler creates the tool handler for ocr_image_url.
func createOCRURLHandler(cfg Config) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		imageURL, err := request.RequireString("url")
		if err != nil {
			return mcp.NewToolResultError("url argument is required"), nil
		}

		// Call ZOCR API
		u, err := url.Parse(fmt.Sprintf("%s/api/ocr/fetch", cfg.ApiURL))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to parse ZOCR fetch URL: %v", err)), nil
		}

		q := u.Query()
		q.Set("url", imageURL)
		u.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to create HTTP request: %v", err)), nil
		}

		if cfg.Token != "" {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.Token))
		}

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to send request to ZOCR API: %v", err)), nil
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return mcp.NewToolResultError(fmt.Sprintf("ZOCR API returned non-OK status %d: %s", resp.StatusCode, string(bodyBytes))), nil
		}

		var zocrResp ZOCRResponse
		if err := json.NewDecoder(resp.Body).Decode(&zocrResp); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to decode ZOCR API response: %v", err)), nil
		}

		if zocrResp.Code != 200 {
			return mcp.NewToolResultError(fmt.Sprintf("ZOCR API error (code %d): %s", zocrResp.Code, zocrResp.Msg)), nil
		}

		// Return both text representation and JSON structured data
		return mcp.NewToolResultStructured(zocrResp.Data, zocrResp.Data.FullText), nil
	}
}

func callZOCRUpload(ctx context.Context, cfg Config, filename string, reader io.Reader) (ZOCRResponse, error) {
	var zocrResp ZOCRResponse

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return zocrResp, fmt.Errorf("failed to create multipart form file: %v", err)
	}

	limited := &io.LimitedReader{R: reader, N: maxImageBytes + 1}
	n, err := io.Copy(part, limited)
	if err != nil {
		return zocrResp, fmt.Errorf("failed to copy image content to multipart request: %v", err)
	}
	if n > maxImageBytes {
		return zocrResp, fmt.Errorf("image exceeds max size of %d bytes", maxImageBytes)
	}
	if err := writer.Close(); err != nil {
		return zocrResp, fmt.Errorf("failed to close multipart writer: %v", err)
	}

	apiURL := fmt.Sprintf("%s/api/ocr/upload", strings.TrimRight(cfg.ApiURL, "/"))
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, body)
	if err != nil {
		return zocrResp, fmt.Errorf("failed to create HTTP request: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	if cfg.Token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.Token))
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return zocrResp, fmt.Errorf("failed to send request to ZOCR API: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return zocrResp, fmt.Errorf("ZOCR API returned non-OK status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if err := json.NewDecoder(resp.Body).Decode(&zocrResp); err != nil {
		return zocrResp, fmt.Errorf("failed to decode ZOCR API response: %v", err)
	}

	if zocrResp.Code != 200 {
		return zocrResp, fmt.Errorf("ZOCR API error (code %d): %s", zocrResp.Code, zocrResp.Msg)
	}

	return zocrResp, nil
}

// decodeBase64Image decodes a base64 encoded image string.
// It strips a data URI prefix and layout whitespace, then decodes standard base64.
func decodeBase64Image(base64Data string) ([]byte, error) {
	if idx := strings.Index(base64Data, ";base64,"); idx != -1 {
		base64Data = base64Data[idx+8:]
	}

	r := strings.NewReplacer("\n", "", "\r", "", "\t", "", " ", "")
	base64Data = r.Replace(base64Data)
	if int64(base64.StdEncoding.DecodedLen(len(base64Data))) > maxImageBytes {
		return nil, fmt.Errorf("image exceeds max size of %d bytes", maxImageBytes)
	}

	decodedBytes, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return nil, err
	}
	if int64(len(decodedBytes)) > maxImageBytes {
		return nil, fmt.Errorf("image exceeds max size of %d bytes", maxImageBytes)
	}
	return decodedBytes, nil
}

func imageExtensionFromBase64(base64Data string) string {
	ext := "png"
	if idx := strings.Index(base64Data, "data:image/"); idx != -1 {
		semiIdx := strings.Index(base64Data[idx+11:], ";")
		if semiIdx != -1 {
			parsedExt := base64Data[idx+11 : idx+11+semiIdx]
			if parsedExt == "jpeg" {
				return "jpg"
			}
			if parsedExt != "" {
				return parsedExt
			}
		}
	}
	return ext
}
