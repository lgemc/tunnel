package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
)

type LogEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
	LogStream string    `json:"log_stream"`
}

// GetLambdaLogs returns recent log events for a Lambda function
func (h *Handler) GetLambdaLogs(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "function name required")
		return
	}

	// Optional query params
	limitStr := r.URL.Query().Get("limit")
	limit := int32(100)
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = int32(l)
		}
	}

	// Optional start time (unix ms)
	var startTime *int64
	if sinceStr := r.URL.Query().Get("since_ms"); sinceStr != "" {
		if ms, err := strconv.ParseInt(sinceStr, 10, 64); err == nil {
			startTime = aws.Int64(ms)
		}
	}
	if startTime == nil {
		// Default: last 30 minutes
		ms := time.Now().Add(-30*time.Minute).UnixMilli()
		startTime = &ms
	}

	ctx := context.Background()
	logGroup := "/aws/lambda/" + name

	// Get the most recent log streams
	streamsOut, err := h.logsClient.DescribeLogStreams(ctx, &cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName: aws.String(logGroup),
		OrderBy:      "LastEventTime",
		Descending:   aws.Bool(true),
		Limit:        aws.Int32(5),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to describe log streams: "+err.Error())
		return
	}

	var events []LogEvent
	for _, stream := range streamsOut.LogStreams {
		if stream.LogStreamName == nil {
			continue
		}
		out, err := h.logsClient.GetLogEvents(ctx, &cloudwatchlogs.GetLogEventsInput{
			LogGroupName:  aws.String(logGroup),
			LogStreamName: stream.LogStreamName,
			StartTime:     startTime,
			StartFromHead: aws.Bool(false),
			Limit:         aws.Int32(limit / int32(len(streamsOut.LogStreams)+1) + 1),
		})
		if err != nil {
			continue
		}
		for _, e := range out.Events {
			msg := ""
			if e.Message != nil {
				msg = *e.Message
			}
			ts := time.Time{}
			if e.Timestamp != nil {
				ts = time.UnixMilli(*e.Timestamp)
			}
			events = append(events, LogEvent{
				Timestamp: ts,
				Message:   msg,
				LogStream: *stream.LogStreamName,
			})
		}
		if int32(len(events)) >= limit {
			break
		}
	}

	// Sort by timestamp descending (newest first)
	for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
		events[i], events[j] = events[j], events[i]
	}

	if int32(len(events)) > limit {
		events = events[:limit]
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"function":  name,
		"log_group": logGroup,
		"events":    events,
		"count":     len(events),
	})
}
