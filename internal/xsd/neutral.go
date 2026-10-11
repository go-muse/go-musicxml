package xsd

const XMLWhitespace = " \t\r\n"

type Attr = struct {
	Name  QName
	Value string
}
type TechnicalError struct{ Message string }

func (e *TechnicalError) Error() string {
	return "validation: incomplete assessment: incomplete XSD assessment: " + e.Message
}
