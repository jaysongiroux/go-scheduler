package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/jaysongiroux/go-scheduler/internal/config"
	"github.com/jaysongiroux/go-scheduler/internal/logger"
)

func ExtractStartEndTs(
	r *http.Request,
) (error, int64, int64) {
	var startTs, endTs int64

	if start := r.URL.Query().Get("start_ts"); start != "" {
		_, err := fmt.Sscanf(start, "%d", &startTs)
		if err != nil {
			logger.Error("Failed to scan start timestamp: %v", err.Error())
			return err, 0, 0
		}
	}
	if end := r.URL.Query().Get("end_ts"); end != "" {
		_, err := fmt.Sscanf(end, "%d", &endTs)
		if err != nil {
			logger.Error("Failed to scan end timestamp: %v", err.Error())
			return err, 0, 0
		}
	}

	if startTs == 0 {
		return errors.New("start_ts is required"), 0, 0
	}
	if endTs == 0 {
		return errors.New("end_ts is required"), 0, 0
	}

	if startTs > endTs {
		return errors.New("start_ts must be before end_ts"), 0, 0
	}

	return nil, startTs, endTs
}

func ExtractLimitOffset(r *http.Request, cfg *config.Config) (error, int, int) {
	limit := r.URL.Query().Get("limit")
	offset := r.URL.Query().Get("offset")

	if limit == "" {
		limit = strconv.Itoa(cfg.DefaultPageSize)
	}
	if offset == "" {
		offset = strconv.Itoa(0)
	}

	limitInt, err := strconv.Atoi(limit)
	if err != nil {
		return errors.New("invalid limit"), 0, 0
	}
	offsetInt, err := strconv.Atoi(offset)
	if err != nil {
		return errors.New("invalid offset"), 0, 0
	}

	return nil, limitInt, offsetInt
}
