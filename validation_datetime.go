package musicxml

import (
	"regexp"
	"strconv"
	"strings"
)

const (
	xsdYearFragment     = `-?([0-9]{4}|[1-9][0-9]{4,})`
	xsdMonthFragment    = `(0[1-9]|1[0-2])`
	xsdDayFragment      = `(0[1-9]|[12][0-9]|3[01])`
	xsdTimezoneFragment = `(?:Z|[+-](?:(?:0[0-9]|1[0-3]):[0-5][0-9]|14:00))?`
	xsdTimeFragment     = `(?:(?:[01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9](?:\.[0-9]+)?|24:00:00(?:\.0+)?)`
)

var (
	xsdDatePattern = regexp.MustCompile(
		`^` + xsdYearFragment + `-` + xsdMonthFragment + `-` +
			xsdDayFragment + xsdTimezoneFragment + `$`,
	)
	xsdDateTimePattern = regexp.MustCompile(
		`^` + xsdYearFragment + `-` + xsdMonthFragment + `-` +
			xsdDayFragment + `T` + xsdTimeFragment +
			xsdTimezoneFragment + `$`,
	)
	xsdTimePattern = regexp.MustCompile(
		`^` + xsdTimeFragment + xsdTimezoneFragment + `$`,
	)
	xsdDurationPattern = regexp.MustCompile(
		`^-?P(?:([0-9]+)Y)?(?:([0-9]+)M)?(?:([0-9]+)D)?` +
			`(?:T(?:([0-9]+)H)?(?:([0-9]+)M)?` +
			`(?:([0-9]+(?:\.[0-9]+)?)S)?)?$`,
	)
	xsdGYearMonthPattern = regexp.MustCompile(
		`^` + xsdYearFragment + `-` + xsdMonthFragment +
			xsdTimezoneFragment + `$`,
	)
	xsdGYearPattern = regexp.MustCompile(
		`^` + xsdYearFragment + xsdTimezoneFragment + `$`,
	)
	xsdGMonthDayPattern = regexp.MustCompile(
		`^--` + xsdMonthFragment + `-` + xsdDayFragment +
			xsdTimezoneFragment + `$`,
	)
	xsdGDayPattern = regexp.MustCompile(
		`^---` + xsdDayFragment + xsdTimezoneFragment + `$`,
	)
	xsdGMonthPattern = regexp.MustCompile(
		`^--` + xsdMonthFragment + xsdTimezoneFragment + `$`,
	)
)

func validXSDDateTime(name string, value string) bool {
	switch name {
	case "date":
		return validXSDDateMatch(xsdDatePattern.FindStringSubmatch(value))

	case "dateTime":
		return validXSDDateMatch(
			xsdDateTimePattern.FindStringSubmatch(value),
		)

	case "duration":
		return validXSDDuration(value)

	case "time":
		return xsdTimePattern.MatchString(value)

	case "gYearMonth":
		match := xsdGYearMonthPattern.FindStringSubmatch(value)
		return len(match) != 0 && validXSDYear(match[1])

	case "gYear":
		match := xsdGYearPattern.FindStringSubmatch(value)
		return len(match) != 0 && validXSDYear(match[1])

	case "gMonthDay":
		match := xsdGMonthDayPattern.FindStringSubmatch(value)
		if len(match) == 0 {
			return false
		}
		month, _ := strconv.Atoi(match[1])
		day, _ := strconv.Atoi(match[2])
		return day <= xsdDaysInMonth(month, true)

	case "gDay":
		return xsdGDayPattern.MatchString(value)

	case "gMonth":
		return xsdGMonthPattern.MatchString(value)

	default:
		return false
	}
}

func validXSDDateMatch(match []string) bool {
	if len(match) == 0 || !validXSDYear(match[1]) {
		return false
	}

	month, _ := strconv.Atoi(match[2])
	day, _ := strconv.Atoi(match[3])
	return day <= xsdDaysInMonth(
		month,
		xsdYearModulo(match[1], 400)%400 == 0 ||
			xsdYearModulo(match[1], 4)%4 == 0 &&
				xsdYearModulo(match[1], 100)%100 != 0,
	)
}

func validXSDYear(value string) bool {
	return strings.Trim(value, "0") != ""
}

func xsdYearModulo(value string, modulus int) int {
	result := 0
	for _, digit := range value {
		result = (result*10 + int(digit-'0')) % modulus
	}

	return result
}

func xsdDaysInMonth(month int, leap bool) int {
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

func validXSDDuration(value string) bool {
	match := xsdDurationPattern.FindStringSubmatch(value)
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
