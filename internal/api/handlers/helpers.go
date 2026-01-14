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
) (int64, int64, error) {
	var startTs, endTs int64

	if start := r.URL.Query().Get("start_ts"); start != "" {
		_, err := fmt.Sscanf(start, "%d", &startTs)
		if err != nil {
			logger.Error("Failed to scan start timestamp: %v", err.Error())
			return 0, 0, err
		}
	}
	if end := r.URL.Query().Get("end_ts"); end != "" {
		_, err := fmt.Sscanf(end, "%d", &endTs)
		if err != nil {
			logger.Error("Failed to scan end timestamp: %v", err.Error())
			return 0, 0, err
		}
	}

	if startTs == 0 {
		return 0, 0, errors.New("start_ts is required")
	}
	if endTs == 0 {
		return 0, 0, errors.New("end_ts is required")
	}

	if startTs > endTs {
		return 0, 0, errors.New("start_ts must be before end_ts")
	}

	return startTs, endTs, nil
}

func ExtractLimitOffset(r *http.Request, cfg *config.Config) (int, int, error) {
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
		return 0, 0, errors.New("invalid limit")
	}
	offsetInt, err := strconv.Atoi(offset)
	if err != nil {
		return 0, 0, errors.New("invalid offset")
	}

	return limitInt, offsetInt, nil
}
