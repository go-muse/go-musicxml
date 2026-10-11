package xsd

import (
	"regexp"
	"strconv"
	"strings"
)

const (
	XsdYearFragment     = `-?([0-9]{4}|[1-9][0-9]{4,})`
	XsdMonthFragment    = `(0[1-9]|1[0-2])`
	XsdDayFragment      = `(0[1-9]|[12][0-9]|3[01])`
	XsdTimezoneFragment = `(?:Z|[+-](?:(?:0[0-9]|1[0-3]):[0-5][0-9]|14:00))?`
	XsdTimeFragment     = `(?:(?:[01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9](?:\.[0-9]+)?|24:00:00(?:\.0+)?)`
)

var (
	XsdDatePattern = regexp.MustCompile(
		`^` + XsdYearFragment + `-` + XsdMonthFragment + `-` +
			XsdDayFragment + XsdTimezoneFragment + `$`,
	)
	XsdDateTimePattern = regexp.MustCompile(
		`^` + XsdYearFragment + `-` + XsdMonthFragment + `-` +
			XsdDayFragment + `T` + XsdTimeFragment +
			XsdTimezoneFragment + `$`,
	)
	XsdTimePattern = regexp.MustCompile(
		`^` + XsdTimeFragment + XsdTimezoneFragment + `$`,
	)
	XsdDurationPattern = regexp.MustCompile(
		`^-?P(?:([0-9]+)Y)?(?:([0-9]+)M)?(?:([0-9]+)D)?` +
			`(?:T(?:([0-9]+)H)?(?:([0-9]+)M)?` +
			`(?:([0-9]+(?:\.[0-9]+)?)S)?)?$`,
	)
	XsdGYearMonthPattern = regexp.MustCompile(
		`^` + XsdYearFragment + `-` + XsdMonthFragment +
			XsdTimezoneFragment + `$`,
	)
	XsdGYearPattern = regexp.MustCompile(
		`^` + XsdYearFragment + XsdTimezoneFragment + `$`,
	)
	XsdGMonthDayPattern = regexp.MustCompile(
		`^--` + XsdMonthFragment + `-` + XsdDayFragment +
			XsdTimezoneFragment + `$`,
	)
	XsdGDayPattern = regexp.MustCompile(
		`^---` + XsdDayFragment + XsdTimezoneFragment + `$`,
	)
	XsdGMonthPattern = regexp.MustCompile(
		`^--` + XsdMonthFragment + XsdTimezoneFragment + `$`,
	)
)

func ValidXSDDateTime(name string, value string) bool {
	switch name {
	case "date":
		return ValidXSDDateMatch(XsdDatePattern.FindStringSubmatch(value))

	case "dateTime":
		return ValidXSDDateMatch(
			XsdDateTimePattern.FindStringSubmatch(value),
		)

	case "duration":
		return ValidXSDDuration(value)

	case "time":
		return XsdTimePattern.MatchString(value)

	case "gYearMonth":
		match := XsdGYearMonthPattern.FindStringSubmatch(value)
		return len(match) != 0 && ValidXSDYear(match[1])

	case "gYear":
		match := XsdGYearPattern.FindStringSubmatch(value)
		return len(match) != 0 && ValidXSDYear(match[1])

	case "gMonthDay":
		match := XsdGMonthDayPattern.FindStringSubmatch(value)
		if len(match) == 0 {
			return false
		}
		month, _ := strconv.Atoi(match[1])
		day, _ := strconv.Atoi(match[2])
		return day <= XsdDaysInMonth(month, true)

	case "gDay":
		return XsdGDayPattern.MatchString(value)

	case "gMonth":
		return XsdGMonthPattern.MatchString(value)

	default:
		return false
	}
}

func ValidXSDDateMatch(match []string) bool {
	if len(match) == 0 || !ValidXSDYear(match[1]) {
		return false
	}

	month, _ := strconv.Atoi(match[2])
	day, _ := strconv.Atoi(match[3])
	return day <= XsdDaysInMonth(
		month,
		XsdYearModulo(match[1], 400)%400 == 0 ||
			XsdYearModulo(match[1], 4)%4 == 0 &&
				XsdYearModulo(match[1], 100)%100 != 0,
	)
}

func ValidXSDYear(value string) bool {
	return strings.Trim(value, "0") != ""
}

func XsdYearModulo(value string, modulus int) int {
	result := 0
	for _, digit := range value {
		result = (result*10 + int(digit-'0')) % modulus
	}

	return result
}

func XsdDaysInMonth(month int, leap bool) int {
	switch month {
	case 2:
		if leap {
			return 29
		}
		return 28
	case 4, 6, 9, 11:
		return 30
	default:
		return 31
	}
}

func ValidXSDDuration(value string) bool {
	match := XsdDurationPattern.FindStringSubmatch(value)
	if len(match) == 0 {
		return false
	}

	hasValue := false
	for index := 1; index < len(match); index++ {
		hasValue = hasValue || match[index] != ""
	}
	if !hasValue {
		return false
	}

	if strings.ContainsRune(value, 'T') {
		return match[4] != "" || match[5] != "" || match[6] != ""
	}

	return true
}
