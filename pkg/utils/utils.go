package utils

import (
	"fmt"
	"strconv"
	"time"

	pb "github.com/etesami/air-quality-monitoring/pkg/protoc"
)

// UnixMilliToTime converts a Unix timestamp in milliseconds to a time.Time object
func UnixMilliToTime(unixMilli int64) time.Time {
	return time.Unix(unixMilli/1000, (unixMilli%1000)*int64(time.Millisecond))
}

// calculateRtt calculates the round-trip time (RTT) based on the current time and the ack time
// It takes the sent message time, received ack time, and the ack object as parameters
func CalculateRtt(sentMsgTime, recAckTime time.Time, ack pb.Ack) (float64, error) {
	sentAckTime, err1 := strconv.Atoi(ack.AckSentTimestamp)
	recMsgTime, err2 := strconv.Atoi(ack.ReceivedTimestamp)
	if err1 != nil || err2 != nil {
		return 0, fmt.Errorf("error converting ack timestamps: %v, %v", err1, err2)
	}
	t1 := int64(recMsgTime) - sentMsgTime.UnixMilli()
	t2 := recAckTime.UnixMilli() - int64(sentAckTime)
	return float64(t1+t2) / 1000.0, nil
}
