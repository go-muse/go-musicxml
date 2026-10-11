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
	return validXSDDateTime(name, value, nil)
}

func validXSDDateTime(name string, value string, c *Context) bool {
	c.scalar(int64(len(value)))
	switch name {
	case "date":
		return validXSDDateMatch(scalarPatternSubmatch(XsdDatePattern, value, c), c)

	case "dateTime":
		return validXSDDateMatch(scalarPatternSubmatch(XsdDateTimePattern, value, c), c)

	case "duration":
		return validXSDDuration(value, c)

	case "time":
		return scalarPatternMatch(XsdTimePattern, value, c)

	case "gYearMonth":
		match := scalarPatternSubmatch(XsdGYearMonthPattern, value, c)
		return len(match) != 0 && validXSDYear(match[1], c)

	case "gYear":
		match := scalarPatternSubmatch(XsdGYearPattern, value, c)
		return len(match) != 0 && validXSDYear(match[1], c)

	case "gMonthDay":
		match := scalarPatternSubmatch(XsdGMonthDayPattern, value, c)
		if len(match) == 0 {
			return false
		}
		month, _ := strconv.Atoi(match[1])
		day, _ := strconv.Atoi(match[2])
		return day <= XsdDaysInMonth(month, true)

	case "gDay":
		return scalarPatternMatch(XsdGDayPattern, value, c)

	case "gMonth":
		return scalarPatternMatch(XsdGMonthPattern, value, c)

	default:
		return false
	}
}

func ValidXSDDateMatch(match []string) bool {
	return validXSDDateMatch(match, nil)
}

func validXSDDateMatch(match []string, c *Context) bool {
	if len(match) == 0 || !validXSDYear(match[1], c) {
		return false
	}

	month, _ := strconv.Atoi(match[2])
	day, _ := strconv.Atoi(match[3])
	return day <= XsdDaysInMonth(
		month,
		xsdYearModulo(match[1], 400, c)%400 == 0 ||
			xsdYearModulo(match[1], 4, c)%4 == 0 &&
				xsdYearModulo(match[1], 100, c)%100 != 0,
	)
}

func ValidXSDYear(value string) bool {
	return validXSDYear(value, nil)
}

func validXSDYear(value string, c *Context) bool {
	for index := range len(value) {
		c.step(1)
		if value[index] != '0' {
			return true
		}
	}
	return false
}

func XsdYearModulo(value string, modulus int) int {
	return xsdYearModulo(value, modulus, nil)
}

func xsdYearModulo(value string, modulus int, c *Context) int {
	result := 0
	for _, digit := range value {
		c.step(1)
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
	return validXSDDuration(value, nil)
}

func validXSDDuration(value string, c *Context) bool {
	match := scalarPatternSubmatch(XsdDurationPattern, value, c)
	if len(match) == 0 {
		return false
	}

	hasValue := false
	for index := 1; index < len(match); index++ {
		c.step(1)
		hasValue = hasValue || match[index] != ""
	}
	if !hasValue {
		return false
	}

	c.bytes(len(value))
	if strings.ContainsRune(value, 'T') {
		return match[4] != "" || match[5] != "" || match[6] != ""
	}

	return true
}
