package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

const expoPushURL = "https://exp.host/--/api/v2/push/send"

// O cliente precisa desistir antes do timeout de 10 segundos da Lambda para que
// o handler ainda consiga devolver a mensagem com falha para a SQS.
var httpClient = &http.Client{Timeout: 8 * time.Second}

type notification struct {
	To    string         `json:"to"`
	Title string         `json:"title"`
	Body  string         `json:"body"`
	Data  map[string]any `json:"data,omitempty"`
}

type expoPushResponse struct {
	Data   expoPushTicket `json:"data"`
	Errors []expoAPIError `json:"errors"`
}

type expoPushTicket struct {
	Status  string           `json:"status"`
	ID      string           `json:"id"`
	Details expoErrorDetails `json:"details"`
}

type expoAPIError struct {
	Code string `json:"code"`
}

type expoErrorDetails struct {
	Error string `json:"error"`
}

func handle(ctx context.Context, event events.SQSEvent) (events.SQSEventResponse, error) {
	result := events.SQSEventResponse{
		BatchItemFailures: []events.SQSBatchItemFailure{},
	}

	for _, message := range event.Records {
		var push notification

		if err := json.Unmarshal([]byte(message.Body), &push); err != nil {
			result.BatchItemFailures = appendFailure(result.BatchItemFailures, message.MessageId)
			continue
		}

		if err := sendToExpo(ctx, push); err != nil {
			result.BatchItemFailures = appendFailure(result.BatchItemFailures, message.MessageId)
		}
	}

	return result, nil
}

func sendToExpo(ctx context.Context, push notification) error {
	body, err := json.Marshal(push)
	if err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		expoPushURL,
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Expo returned HTTP %d", response.StatusCode)
	}

	// O Expo pode responder HTTP 200 e, ainda assim, rejeitar a notificacao no
	// ticket. Verificar somente o status HTTP faria a SQS apagar uma mensagem que
	// nunca foi aceita pelo Expo.
	var pushResponse expoPushResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&pushResponse); err != nil {
		return fmt.Errorf("invalid Expo response: %w", err)
	}
	if len(pushResponse.Errors) > 0 {
		return fmt.Errorf("Expo rejected the request: %s", pushResponse.Errors[0].Code)
	}
	if pushResponse.Data.Status != "ok" || pushResponse.Data.ID == "" {
		return fmt.Errorf("Expo rejected the notification: %s", pushResponse.Data.Details.Error)
	}

	return nil
}

func appendFailure(
	failures []events.SQSBatchItemFailure,
	messageID string,
) []events.SQSBatchItemFailure {
	return append(failures, events.SQSBatchItemFailure{ItemIdentifier: messageID})
}

func main() {
	lambda.Start(handle)
}
