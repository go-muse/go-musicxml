# MusicXML interoperability corpus

`musicxml-test-suite` is a snapshot of the MIT-licensed
[MusicXML Test Suite](https://github.com/w3c-cg/musicxmlTestSuite) at commit
`b2e6a1627b8574c9714e1fd0a8a5b1921e10f8f3`, retrieved on 2026-07-26.

The snapshot contains 149 XML documents and one compressed `.mxl` package.
Some documents are deliberately invalid according to the MusicXML schema or
describe ambiguous notation. They are still useful transport round-trip and
fuzzing inputs; XSD validity is tested separately.

The upstream `LICENSE` and `UPSTREAM_README.md` are preserved alongside the
fixtures.
