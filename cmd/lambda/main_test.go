package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (roundTripper roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTripper(request)
}

func TestHandleSendsValidMessage(t *testing.T) {
	originalClient := httpClient
	t.Cleanup(func() { httpClient = originalClient })

	httpClient = responseHTTPClient(
		http.StatusOK,
		`{"data":{"status":"ok","id":"ticket-1"}}`,
	)

	result, err := handle(context.Background(), queueEvent())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.BatchItemFailures) != 0 {
		t.Fatal("expected message to be sent successfully")
	}
	if result.BatchItemFailures == nil {
		t.Fatal("expected an empty failure list instead of null")
	}
}

func TestHandleReturnsFailedMessageToSQS(t *testing.T) {
	originalClient := httpClient
	t.Cleanup(func() { httpClient = originalClient })

	httpClient = responseHTTPClient(http.StatusServiceUnavailable, "")

	result, err := handle(context.Background(), queueEvent())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.BatchItemFailures) != 1 {
		t.Fatal("expected failed message to return to SQS")
	}
}

func TestHandleReturnsExpoTicketErrorToSQS(t *testing.T) {
	originalClient := httpClient
	t.Cleanup(func() { httpClient = originalClient })

	httpClient = responseHTTPClient(
		http.StatusOK,
		`{"data":{"status":"error","details":{"error":"DeviceNotRegistered"}}}`,
	)

	result, err := handle(context.Background(), queueEvent())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.BatchItemFailures) != 1 {
		t.Fatal("expected Expo ticket error to return the message to SQS")
	}
}

func TestHandleReturnsInvalidExpoResponseToSQS(t *testing.T) {
	originalClient := httpClient
	t.Cleanup(func() { httpClient = originalClient })

	httpClient = responseHTTPClient(http.StatusOK, `not-json`)

	result, err := handle(context.Background(), queueEvent())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.BatchItemFailures) != 1 {
		t.Fatal("expected invalid Expo response to return the message to SQS")
	}
}

func responseHTTPClient(statusCode int, body string) *http.Client {
	return &http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: statusCode,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})}
}

func queueEvent() events.SQSEvent {
	return events.SQSEvent{Records: []events.SQSMessage{
		{
			MessageId: "message-1",
			Body:      `{"to":"ExpoPushToken[test]","title":"Novo evento","body":"Evento criado"}`,
		},
	}}
}
