package handler

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

func DecompressBody(body []byte) ([]byte, error) {
	gzipReader, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	decompressedData, err := io.ReadAll(gzipReader)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress data: %w", err)
	}
	return decompressedData, nil
}

func ReadRequestBody(r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}
	r.Body.Close()
	return body, nil
}

func HandleDecompression(r *http.Request) ([]byte, error) {
	// Read raw body
	body, err := ReadRequestBody(r)
	if err != nil {
		return nil, err
	}

	// Handle decompression
	var processData []byte
	if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
		processData, err = DecompressBody(body)
		if err != nil {
			return nil, err
		}
	} else {
		processData = body
	}

	return processData, nil
}

func isIntegerAtoi(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

// isValidLuhn validates a number using the Luhn algorithm
func isValidLuhn(number string) bool {

	if !isIntegerAtoi(number) {
		return false
	}

	sum := 0
	doubleDigit := false

	// Process from right to left
	for i := len(number) - 1; i >= 0; i-- {
		digit := int(number[i] - '0')

		if doubleDigit {
			digit *= 2
			if digit > 9 {
				digit = digit - 9
			}
		}

		sum += digit
		doubleDigit = !doubleDigit
	}

	return sum%10 == 0
}
