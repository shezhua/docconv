package main

import (
	"cloud.google.com/go/errorreporting"
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"

	"code.sajari.com/docconv"
	"code.sajari.com/docconv/docd/internal"
)

var (
	listenAddr = flag.String("addr", ":8888", "The address to listen on (e.g. 127.0.0.1:8888)")

	inputPath = flag.String("input", "", "The file path to convert and exit; no server")

	languages = flag.String("lang", "eng,chi_sim", "Set OCR language list")

	errorReporting                 = flag.Bool("error-reporting", false, "Whether or not to enable GCP Error Reporting")
	errorReportingGCPProjectID     = flag.String("error-reporting-gcp-project-id", "", "The GCP project to use for Error Reporting")
	errorReportingAppEngineService = flag.String("error-reporting-app-engine-service", "", "The App Engine service to use for Error Reporting")

	logLevel = flag.Uint("log-level", 0, "The verbosity of the log")

	readabilityLengthLow          = flag.Int("readability-length-low", 70, "Sets the readability length low")
	readabilityLengthHigh         = flag.Int("readability-length-high", 200, "Sets the readability length high")
	readabilityStopwordsLow       = flag.Float64("readability-stopwords-low", 0.2, "Sets the readability stopwords low")
	readabilityStopwordsHigh      = flag.Float64("readability-stopwords-high", 0.3, "Sets the readability stopwords high")
	readabilityMaxLinkDensity     = flag.Float64("readability-max-link-density", 0.2, "Sets the readability max link density")
	readabilityMaxHeadingDistance = flag.Int("readability-max-heading-distance", 200, "Sets the readability max heading distance")
	readabilityUseClasses         = flag.String("readability-use-classes", "good,neargood", "Comma separated list of readability classes to use")
)

func main() {
	flag.Parse()

	var er internal.ErrorReporter = &internal.NopErrorReporter{}
	if *errorReporting {
		if *errorReportingGCPProjectID == "" {
			*errorReportingGCPProjectID = os.Getenv("GOOGLE_CLOUD_PROJECT")
		}
		if *errorReportingAppEngineService == "" {
			*errorReportingAppEngineService = os.Getenv("GAE_SERVICE")
		}
		var err error
		er, err = errorreporting.NewClient(context.Background(), *errorReportingGCPProjectID, errorreporting.Config{
			ServiceName: *errorReportingAppEngineService,
			OnError: func(err error) {
				log.Printf("Could not report error to Error Reporting service: %v", err)
			},
		})
		if err != nil {
			log.Fatalf("Could not create Error Reporting client: %v", err)
		}
	}

	cs := &convertServer{
		er: er,
	}

	// TODO: Improve this (remove the need for it!)
	docconv.HTMLReadabilityOptionsValues = docconv.HTMLReadabilityOptions{
		LengthLow:             *readabilityLengthLow,
		LengthHigh:            *readabilityLengthHigh,
		StopwordsLow:          *readabilityStopwordsLow,
		StopwordsHigh:         *readabilityStopwordsHigh,
		MaxLinkDensity:        *readabilityMaxLinkDensity,
		MaxHeadingDistance:    *readabilityMaxHeadingDistance,
		ReadabilityUseClasses: *readabilityUseClasses,
	}

	if languages != nil && *languages != "" {
		log.Printf("Set languages: %s\n", *languages)
		langSet := strings.Split(*languages, ",")
		docconv.SetImageLanguages(langSet...)
	}

	if *inputPath != "" {
		resp, err := docconv.ConvertPath(*inputPath)
		if err != nil {
			log.Printf("error converting file '%v': %v", *inputPath, err)
		}
		fmt.Print(string(resp.Body))
		return
	}

	serve(er, cs)
}

// Start the conversion web service
func serve(er internal.ErrorReporter, cs *convertServer) {
	r := mux.NewRouter()
	r.HandleFunc("/convert", cs.convert)

	// Start webserver
	log.Println("Setting log level to", *logLevel)
	log.Println("Starting docconv on", *listenAddr)
	log.Fatal(http.ListenAndServe(*listenAddr, internal.RecoveryHandler(er)(r)))
}
