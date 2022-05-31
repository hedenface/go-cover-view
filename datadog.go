package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/DataDog/datadog-api-client-go/api/v2/datadog"
)

func submitDataDog(coveragePercentage float64) {

	vertical := os.Getenv("VERTICAL")
	if vertical == "" {
		fmt.Println("env VERTICAL not set! Must be one of BP, CP, or IP!")
		return
	}
	if vertical != "BP" && vertical != "CP" && vertical != "IP" {
		fmt.Printf("env VERTICAL set to '%s'! Must be one of BP, CP, or IP!\n", vertical)
		return	
	}

	project := os.Getenv("PROJECT")
	if vertical == "" {
		fmt.Println("env PROJECT not set!")
		return
	}

	projectType := os.Getenv("PROJECT_TYPE")
	if projectType == "" {
		fmt.Println("env PROJECT_TYPE not set! Must be one of BP, CP, or IP!")
		return
	}
	if projectType != "Backend" && projectType != "Frontend" && vertical != "Other" {
		fmt.Printf("env PROJECT_TYPE set to '%s'! Must be one of Backend, Frontend, or Other!\n", projectType)
		return	
	}

	body := datadog.MetricPayload{
		Series: []datadog.MetricSeries{
			{
				Metric: "code.coverage",
				Type:   datadog.METRICINTAKETYPE_UNSPECIFIED.Ptr(),
				Points: []datadog.MetricPoint{
					{
						Timestamp: datadog.PtrInt64(time.Now().Unix()),
						Value:     datadog.PtrFloat64(coveragePercentage),
					},
				},
				Tags: []string{
					fmt.Sprintf("Project:%s", project),
					fmt.Sprintf("ProjectType:%s", projectType),
					fmt.Sprintf("Vertical:%s", vertical),
				},
			},
		},
	}

	ctx := datadog.NewDefaultContext(context.Background())
	configuration := datadog.NewConfiguration()
	apiClient := datadog.NewAPIClient(configuration)
	resp, r, err := apiClient.MetricsApi.SubmitMetrics(ctx, body, *datadog.NewSubmitMetricsOptionalParameters())

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error when calling `MetricsApi.SubmitMetrics`: %v\n", err)
		fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
	}

	responseContent, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Fprintf(os.Stdout, "Response from `MetricsApi.SubmitMetrics`:\n%s\n", responseContent)
}
