package main

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOCRUploadHandlerForwardsFile(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/ocr/upload" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("unexpected authorization header: %q", got)
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("missing file: %v", err)
		}
		defer file.Close()

		if header.Filename != "sample.png" {
			t.Fatalf("unexpected filename: %q", header.Filename)
		}
		body, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("read uploaded file: %v", err)
		}
		if string(body) != "image-bytes" {
			t.Fatalf("unexpected file body: %q", string(body))
		}

		var resp ZOCRResponse
		resp.Code = 200
		resp.Msg = "success"
		resp.Data.FullText = "hello"
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer upstream.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "sample.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("image-bytes")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/ocr/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	createOCRUploadHandler(Config{
		ApiURL: upstream.URL,
		Token:  "test-token",
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body: %s", rec.Code, rec.Body.String())
	}

	var got ZOCRResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Data.FullText != "hello" {
		t.Fatalf("unexpected OCR text: %q", got.Data.FullText)
	}
}
