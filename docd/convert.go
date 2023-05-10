package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"

	"cloud.google.com/go/errorreporting"

	"code.sajari.com/docconv"
	"code.sajari.com/docconv/docd/internal"
)

type convertServer struct {
	er internal.ErrorReporter
}

var (
	name   = regexp.MustCompile("([^\\s]+)的简历")
	email  = regexp.MustCompile("(?i)[a-z0-9]+@[a-z0-9]+(\\.[a-z]+)?")
	phone  = regexp.MustCompile("(\\+\\d+)?(1[0-9]{10}|([0-9]{3,}[ -][0-9]{3,}[ -][0-9]{3,}))")
	degree = regexp.MustCompile("(大专|专科|本科|学士|硕士|研究生|博士|博士后)")
	school = regexp.MustCompile(`\p{Han}{2,}(学校|大学|学院)`)
)

func (s *convertServer) convert(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Readability flag. Currently only used for HTML
	var readability bool
	if r.FormValue("readability") == "1" {
		readability = true
		if *logLevel >= 2 {
			log.Println("Readability is on")
		}
	}

	path := r.FormValue("path")
	if path != "" {
		mimeType := docconv.MimeTypeByExtension(path)

		f, err := os.Open(path)
		if err != nil {
			s.serverError(ctx, w, r, fmt.Errorf("could not open file: %w", err))
			return
		}
		defer f.Close()

		data, err := docconv.Convert(f, mimeType, readability)
		if err != nil {
			s.serverError(ctx, w, r, fmt.Errorf("could not convert file from path %v: %w", path, err))
			return
		}

		s.respond(ctx, w, r, http.StatusOK, data)
		return
	}

	// Get uploaded file
	file, info, err := r.FormFile("input")
	if err != nil {
		s.serverError(ctx, w, r, fmt.Errorf("could not get input file: %w", err))
		return
	}
	defer file.Close()

	// Abort if file doesn't have a mime type
	if len(info.Header["Content-Type"]) == 0 {
		s.clientError(ctx, w, r, http.StatusUnprocessableEntity, "input file %v does not have a Content-Type header", info.Filename)
		return
	}

	// If a generic mime type was provided then use file extension to determine mimetype
	mimeType := info.Header["Content-Type"][0]
	if mimeType == "application/octet-stream" {
		mimeType = docconv.MimeTypeByExtension(info.Filename)
	}

	if *logLevel >= 1 {
		log.Printf("Received file: %v (%v)", info.Filename, mimeType)
	}

	data, err := docconv.Convert(file, mimeType, readability)
	if err != nil {
		s.serverError(ctx, w, r, fmt.Errorf("could not convert file: %w", err))
		return
	}
	//抽取关键信息
	data.Info = extractInfo(info.Filename, data.Body, configLoad(r))

	s.respond(ctx, w, r, http.StatusOK, data)
}

func configLoad(request *http.Request) map[string]*regexp.Regexp {
	config := map[string]*regexp.Regexp{
		"name":   name,
		"phone":  phone,
		"email":  email,
		"degree": degree,
		"school": school,
	}
	keys := []string{"name", "phone", "school", "degree", "birthday", "title", "company"}
	prefix := "exp-"
	compile := regexp.MustCompile(prefix)
	for _, key := range keys {
		value := request.FormValue(prefix + key)
		if value != "" {
			exp := regexp.MustCompile(value)
			if exp != nil {
				field := compile.ReplaceAllString(key, "")
				config[field] = exp
			}
		}
	}
	return config
}

func (s *convertServer) clientError(ctx context.Context, w http.ResponseWriter, r *http.Request, code int, pattern string, args ...interface{}) {
	s.respond(ctx, w, r, code, &docconv.Response{
		Error: fmt.Sprintf(pattern, args...),
	})

	log.Printf(pattern, args...)
}

func (s *convertServer) serverError(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(`{"error":"internal server error"}`))

	e := errorreporting.Entry{
		Error: err,
		Req:   r,
	}
	s.er.Report(e)

	log.Printf("%v", err)
}

func (s *convertServer) respond(ctx context.Context, w http.ResponseWriter, r *http.Request, code int, resp interface{}) {
	buf := &bytes.Buffer{}
	err := json.NewEncoder(buf).Encode(resp)
	if err != nil {
		s.serverError(ctx, w, r, fmt.Errorf("could not marshal JSON response: %w", err))
		return
	}
	w.WriteHeader(code)
	n, err := io.Copy(w, buf)
	if err != nil {
		panic(fmt.Errorf("could not write to response (failed after %d bytes): %w", n, err))
	}
}
