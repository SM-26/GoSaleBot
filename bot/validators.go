package bot

import (
	"database/sql"
	"gosalebot/db"
	"gosalebot/i18n"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// Validator function types
type PriceValidator func(dbConn *sql.DB, price string, lang string) (bool, string)
type PhotosValidator func(dbConn *sql.DB, photos []string, lang string) (bool, string)

var (
	mu              sync.RWMutex
	priceValidator  PriceValidator  = defaultPriceValidator
	photosValidator PhotosValidator = defaultPhotosValidator
)

// RegisterPriceValidator allows injecting a custom PriceValidator.
func RegisterPriceValidator(v PriceValidator) {
	mu.Lock()
	defer mu.Unlock()
	if v != nil {
		priceValidator = v
	}
}

// RegisterPhotosValidator allows injecting a custom PhotosValidator.
func RegisterPhotosValidator(v PhotosValidator) {
	mu.Lock()
	defer mu.Unlock()
	if v != nil {
		photosValidator = v
	}
}

// ValidatePrice is the pluggable entrypoint.
func ValidatePrice(dbConn *sql.DB, price string, lang string) (bool, string) {
	mu.RLock()
	v := priceValidator
	mu.RUnlock()
	return v(dbConn, price, lang)
}

// ValidatePhotos is the pluggable entrypoint.
func ValidatePhotos(dbConn *sql.DB, photos []string, lang string) (bool, string) {
	mu.RLock()
	v := photosValidator
	mu.RUnlock()
	return v(dbConn, photos, lang)
}

// defaultPriceValidator strips currency symbols and thousand separators then parses float.
func defaultPriceValidator(dbConn *sql.DB, price string, lang string) (bool, string) {
	enabled := true
	if dbConn != nil {
		if cfg, err := db.GetConfig(dbConn, "VALIDATE_PRICE"); err == nil && cfg != "" {
			lower := strings.ToLower(cfg)
			if lower == "0" || lower == "false" || lower == "no" {
				enabled = false
			}
		}
	}
	if !enabled {
		return true, ""
	}
	s := strings.TrimSpace(price)
	if s == "" {
		return false, i18n.T(lang, "price_empty")
	}
	// remove currency symbols and spaces
	// allow digits, dots, commas, and minus sign
	// remove common currency symbols
	s = strings.ReplaceAll(s, "€", "")
	s = strings.ReplaceAll(s, "$", "")
	s = strings.ReplaceAll(s, "£", "")
	s = strings.ReplaceAll(s, "₪", "")
	s = strings.TrimSpace(s)
	// remove thousands separators (commas or spaces) but keep decimal comma
	// Heuristic: if both comma and dot present, remove commas as thousand separators
	if strings.Contains(s, ".") && strings.Contains(s, ",") {
		s = strings.ReplaceAll(s, ",", "")
	}
	// remove spaces and apostrophes as thousand separators
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "'", "")
	// replace comma with dot for decimal
	s = strings.ReplaceAll(s, ",", ".")
	// now s should be parsable as float
	// ensure only digits, dot, optional leading +/-, else fail
	matched, _ := regexp.MatchString(`^[+-]?[0-9]*\.?[0-9]+$`, s)
	if !matched {
		return false, i18n.T(lang, "price_invalid")
	}
	if _, err := strconv.ParseFloat(s, 64); err != nil {
		return false, i18n.T(lang, "price_invalid")
	}
	return true, ""
}

// defaultPhotosValidator ensures at least MIN_PHOTOS exist (default 1).
func defaultPhotosValidator(dbConn *sql.DB, photos []string, lang string) (bool, string) {
	min := 1
	if dbConn != nil {
		if cfg, err := db.GetConfig(dbConn, "MIN_PHOTOS"); err == nil && cfg != "" {
			if v, err2 := strconv.Atoi(cfg); err2 == nil {
				min = v
			}
		}
	}
	if len(photos) < min {
		return false, i18n.T(lang, "photos_too_few", min)
	}
	return true, ""
}
