package musicxml

//go:generate go run ./internal/xsdgen/cmd/xsdgen -kind simple -schema-dir schema/musicxml-4.0 -schema musicxml.xsd -catalog catalog.xml -package musicxml -output zz_generated_simple.go
//go:generate go run ./internal/xsdgen/cmd/xsdgen -kind complex -schema-dir schema/musicxml-4.0 -schema musicxml.xsd -catalog catalog.xml -package musicxml -ordered-type credit -ordered-type lyric -ordered-type metronome -output zz_generated_types.go
//go:generate go run ./internal/xsdgen/cmd/xsdgen -kind element -schema-dir schema/musicxml-4.0 -schema musicxml.xsd -catalog catalog.xml -package musicxml -element score-partwise -element score-timewise -output zz_generated_documents.go
//go:generate go run ./internal/xsdgen/cmd/xsdgen -kind document -schema-dir schema/musicxml-4.0 -schema opus.xsd -catalog catalog.xml -package musicxml -element opus -element-go-name OpusDocument -type-name score=OpusScore -external-type yes-no -output zz_generated_opus.go
//go:generate go run ./internal/xsdgen/cmd/xsdgen -kind validation -schema-dir schema/musicxml-4.0 -schema musicxml.xsd -catalog catalog.xml -package musicxml -validation-name scoreValidationSchema -output zz_generated_validation.go
//go:generate go run ./internal/xsdgen/cmd/xsdgen -kind validation -schema-dir schema/musicxml-4.0 -schema opus.xsd -catalog catalog.xml -package musicxml -validation-name opusValidationSchema -output zz_generated_opus_validation.go
